// 这个文件实现上行音频的混合 VAD：先做本地能量门控，再在边界阶段使用 Silero 复核。

package processor

import (
	"context"
	"math"
	"sync"
	"time"

	vadoutbound "gopbx/internal/adapter/outbound/vad"
	"gopbx/internal/compat"
	mediaentity "gopbx/internal/domain/media"
	"gopbx/internal/domain/protocol"
	"gopbx/pkg/wsproto"
)

const (
	defaultVADSampleRate           = 16000
	defaultVADFrameMs              = 20
	defaultVADPreRollMs            = 200
	defaultVADSilencePaddingMs     = 600
	defaultVADTailPaddingMs        = 160
	defaultVADStartWindowMs        = 60
	defaultVADEndWindowMs          = 200
	defaultVADMaxSegmentMs         = 12000
	defaultVADEnterRatio           = 2.2
	defaultVADKeepRatio            = 1.6
	defaultVADStartActivityRatio   = 0.6
	defaultVADEndActivityRatio     = 0.2
	defaultVADResumeActivityRatio  = 0.4
	defaultVADMinEnterRMS          = 120
	defaultVADMinKeepRMS           = 96
	defaultVADSileroStartThreshold = 0.65
	defaultVADSileroEndThreshold   = 0.35
	defaultVADScoreTimeout         = 300 * time.Millisecond
	defaultVADNoiseAlpha           = 0.1
)

type vadState string

const (
	vadStateIdle           vadState = "idle"
	vadStateCandidateStart vadState = "candidateStart"
	vadStateSpeaking       vadState = "speaking"
	vadStateCandidateEnd   vadState = "candidateEnd"
)

type bufferedAudio struct {
	frames  [][]byte
	lengths []int
	totalMs int
}

type activityEntry struct {
	durationMs int
	active     bool
}

type activityWindow struct {
	entries  []activityEntry
	totalMs  int
	activeMs int
}

func (b *bufferedAudio) Push(payload []byte, durationMs int, limitMs int) {
	if len(payload) == 0 || durationMs <= 0 {
		return
	}
	clone := append([]byte(nil), payload...)
	b.frames = append(b.frames, clone)
	b.lengths = append(b.lengths, durationMs)
	b.totalMs += durationMs
	if limitMs <= 0 {
		return
	}
	for b.totalMs > limitMs && len(b.frames) > 0 {
		b.totalMs -= b.lengths[0]
		b.frames = b.frames[1:]
		b.lengths = b.lengths[1:]
	}
}

func (b *bufferedAudio) Reset() {
	b.frames = nil
	b.lengths = nil
	b.totalMs = 0
}

func (b *bufferedAudio) Append(other bufferedAudio, limitMs int) {
	for i, frame := range other.frames {
		b.Push(frame, other.lengths[i], limitMs)
	}
}

func (b *bufferedAudio) Packets(trackID string) []mediaentity.Packet {
	packets := make([]mediaentity.Packet, 0, len(b.frames))
	for _, frame := range b.frames {
		packets = append(packets, mediaentity.Packet{TrackID: trackID, Data: append([]byte(nil), frame...), Kind: mediaentity.PacketKindAudio})
	}
	return packets
}

func (b *bufferedAudio) PrefixPackets(trackID string, maxMs int) []mediaentity.Packet {
	if maxMs <= 0 {
		return nil
	}
	packets := make([]mediaentity.Packet, 0, len(b.frames))
	usedMs := 0
	for i, frame := range b.frames {
		packets = append(packets, mediaentity.Packet{TrackID: trackID, Data: append([]byte(nil), frame...), Kind: mediaentity.PacketKindAudio})
		usedMs += b.lengths[i]
		if usedMs >= maxMs {
			break
		}
	}
	return packets
}

func (b *bufferedAudio) Bytes() []byte {
	total := 0
	for _, frame := range b.frames {
		total += len(frame)
	}
	joined := make([]byte, 0, total)
	for _, frame := range b.frames {
		joined = append(joined, frame...)
	}
	return joined
}

func (w *activityWindow) Push(active bool, durationMs int, limitMs int) {
	if durationMs <= 0 {
		return
	}
	w.entries = append(w.entries, activityEntry{durationMs: durationMs, active: active})
	w.totalMs += durationMs
	if active {
		w.activeMs += durationMs
	}
	if limitMs <= 0 {
		return
	}
	for w.totalMs > limitMs && len(w.entries) > 0 {
		entry := w.entries[0]
		w.totalMs -= entry.durationMs
		if entry.active {
			w.activeMs -= entry.durationMs
		}
		w.entries = w.entries[1:]
	}
}

func (w *activityWindow) Reset() {
	w.entries = nil
	w.totalMs = 0
	w.activeMs = 0
}

func (w activityWindow) Clone() activityWindow {
	clone := activityWindow{totalMs: w.totalMs, activeMs: w.activeMs}
	if len(w.entries) > 0 {
		clone.entries = append([]activityEntry(nil), w.entries...)
	}
	return clone
}

func (w activityWindow) Ratio() float64 {
	if w.totalMs <= 0 {
		return 0
	}
	return float64(w.activeMs) / float64(w.totalMs)
}

type VAD struct {
	mu sync.Mutex

	mode                 string
	scorer               vadoutbound.Scorer
	sampleRate           int
	frameMs              int
	preRollMs            int
	startWindowMs        int
	endWindowMs          int
	silencePaddingMs     int
	tailPaddingMs        int
	maxSegmentMs         int
	enterRatio           float64
	keepRatio            float64
	startActivityRatio   float64
	endActivityRatio     float64
	resumeActivityRatio  float64
	minEnterRMS          float64
	minKeepRMS           float64
	sileroStartThreshold float32
	sileroEndThreshold   float32
	scoreTimeout         time.Duration

	state             vadState
	noiseFloor        float64
	noiseReady        bool
	preRoll           bufferedAudio
	startBuffer       bufferedAudio
	tailBuffer        bufferedAudio
	startActivity     activityWindow
	speechActivity    activityWindow
	segmentStartMs    int64
	segmentDurationMs int
}

func NewVAD(cfg *wsproto.VADOption, scorer vadoutbound.Scorer) *VAD {
	mode := vadoutbound.NormalizeType(derefVADString(cfg, func(v *wsproto.VADOption) *string { return v.Type }))
	if mode == "" {
		mode = vadoutbound.TypePassthrough
	}
	silencePaddingMs := int(valueOrUint64(cfg, func(v *wsproto.VADOption) *uint64 { return v.SilencePadding }, defaultVADSilencePaddingMs))
	startWindowMs := maxInt(defaultVADStartWindowMs, defaultVADFrameMs*3)
	endWindowMs := minInt(defaultVADEndWindowMs, silencePaddingMs)
	if endWindowMs < defaultVADFrameMs*3 {
		endWindowMs = defaultVADFrameMs * 3
	}
	return &VAD{
		mode:                 mode,
		scorer:               scorer,
		sampleRate:           int(valueOrUint32(cfg, func(v *wsproto.VADOption) *uint32 { return v.Samplerate }, defaultVADSampleRate)),
		frameMs:              defaultVADFrameMs,
		preRollMs:            int(valueOrUint64(cfg, func(v *wsproto.VADOption) *uint64 { return v.SpeechPadding }, defaultVADPreRollMs)),
		startWindowMs:        startWindowMs,
		endWindowMs:          endWindowMs,
		silencePaddingMs:     silencePaddingMs,
		tailPaddingMs:        defaultVADTailPaddingMs,
		maxSegmentMs:         int(valueOrUint64(cfg, func(v *wsproto.VADOption) *uint64 { return v.MaxBufferDurationSecs }, defaultVADMaxSegmentMs/1000) * 1000),
		enterRatio:           float64(valueOrFloat32(cfg, func(v *wsproto.VADOption) *float32 { return v.Ratio }, defaultVADEnterRatio)),
		keepRatio:            defaultVADKeepRatio,
		startActivityRatio:   defaultVADStartActivityRatio,
		endActivityRatio:     defaultVADEndActivityRatio,
		resumeActivityRatio:  defaultVADResumeActivityRatio,
		minEnterRMS:          defaultVADMinEnterRMS,
		minKeepRMS:           defaultVADMinKeepRMS,
		sileroStartThreshold: valueOrFloat32(cfg, func(v *wsproto.VADOption) *float32 { return v.VoiceThreshold }, defaultVADSileroStartThreshold),
		sileroEndThreshold:   defaultVADSileroEndThreshold,
		scoreTimeout:         defaultVADScoreTimeout,
		state:                vadStateIdle,
	}
}

func (v *VAD) Name() string { return "vad" }

func (v *VAD) Close() error {
	if v == nil || v.scorer == nil {
		return nil
	}
	closer, ok := v.scorer.(interface{ Close() error })
	if !ok {
		return nil
	}
	return closer.Close()
}

func (v *VAD) Process(packet mediaentity.Packet) Result {
	packet.Kind = packet.ResolvedKind()
	if packet.Kind != mediaentity.PacketKindAudio {
		return passthrough(packet)
	}
	if len(packet.Data) == 0 {
		return Result{}
	}
	if v.mode == vadoutbound.TypePassthrough {
		return passthrough(packet)
	}

	durationMs := pcmDurationMs(packet.Data, v.sampleRate)
	if durationMs <= 0 {
		return Result{}
	}
	rms := rmsPCM16LE(packet.Data)

	v.mu.Lock()
	defer v.mu.Unlock()

	switch v.state {
	case vadStateIdle:
		return v.processIdleLocked(packet, durationMs, rms)
	case vadStateCandidateStart:
		return v.processCandidateStartLocked(packet, durationMs, rms)
	case vadStateSpeaking:
		return v.processSpeakingLocked(packet, durationMs, rms)
	case vadStateCandidateEnd:
		return v.processCandidateEndLocked(packet, durationMs, rms)
	default:
		v.resetToIdleLocked()
		return Result{}
	}
}

func (v *VAD) processIdleLocked(packet mediaentity.Packet, durationMs int, rms float64) Result {
	active := v.enterHitLocked(rms)
	if !active {
		v.observeNoiseLocked(rms)
		v.preRoll.Push(packet.Data, durationMs, v.preRollMs)
		return Result{}
	}
	v.state = vadStateCandidateStart
	v.startBuffer.Reset()
	v.startActivity.Reset()
	v.startBuffer.Push(packet.Data, durationMs, v.startWindowMs)
	v.startActivity.Push(active, durationMs, v.startWindowMs)
	return v.maybeConfirmStartLocked(packet.TrackID)
}

func (v *VAD) processCandidateStartLocked(packet mediaentity.Packet, durationMs int, rms float64) Result {
	active := v.enterHitLocked(rms)
	if !active {
		v.observeNoiseLocked(rms)
	}
	v.startBuffer.Push(packet.Data, durationMs, v.startWindowMs)
	v.startActivity.Push(active, durationMs, v.startWindowMs)
	return v.maybeConfirmStartLocked(packet.TrackID)
}

func (v *VAD) processSpeakingLocked(packet mediaentity.Packet, durationMs int, rms float64) Result {
	active := v.keepHitLocked(rms)
	if !active {
		v.observeNoiseLocked(rms)
	}
	v.speechActivity.Push(active, durationMs, v.endWindowMs)
	if v.shouldEnterCandidateEndLocked() {
		v.state = vadStateCandidateEnd
		v.tailBuffer.Reset()
		v.tailBuffer.Push(packet.Data, durationMs, 0)
		return Result{}
	}
	v.segmentDurationMs += durationMs
	return v.maybeForceSegmentEndLocked(packet.TrackID, Result{Packets: []mediaentity.Packet{packet}})
}

func (v *VAD) processCandidateEndLocked(packet mediaentity.Packet, durationMs int, rms float64) Result {
	active := v.keepHitLocked(rms)
	if !active {
		v.observeNoiseLocked(rms)
	}
	v.speechActivity.Push(active, durationMs, v.endWindowMs)
	v.tailBuffer.Push(packet.Data, durationMs, 0)
	if v.shouldResumeSpeakingLocked() {
		v.state = vadStateSpeaking
		packets := v.tailBuffer.Packets(packet.TrackID)
		v.segmentDurationMs += v.tailBuffer.totalMs
		v.tailBuffer.Reset()
		return v.maybeForceSegmentEndLocked(packet.TrackID, Result{Packets: packets})
	}
	if v.tailBuffer.totalMs < v.silencePaddingMs {
		return Result{}
	}
	if !v.shouldConfirmEndLocked() {
		return Result{}
	}
	return v.confirmEndLocked(packet.TrackID)
}

func (v *VAD) maybeConfirmStartLocked(trackID string) Result {
	if v.startBuffer.totalMs < v.startWindowMs || v.startActivity.totalMs < v.startWindowMs {
		return Result{}
	}
	if v.startActivity.Ratio() < v.startActivityRatio {
		if v.startActivity.activeMs == 0 {
			v.preRoll.Append(v.startBuffer, v.preRollMs)
			v.state = vadStateIdle
			v.startBuffer.Reset()
			v.startActivity.Reset()
		}
		return Result{}
	}
	accepted := true
	if v.mode == vadoutbound.TypeHybrid && v.scorer != nil {
		score, err := v.scoreLocked(v.startWindowBytesLocked())
		if err == nil {
			accepted = score >= v.sileroStartThreshold
		}
	}
	if !accepted {
		v.preRoll.Append(v.startBuffer, v.preRollMs)
		v.state = vadStateIdle
		v.startBuffer.Reset()
		v.startActivity.Reset()
		return Result{}
	}
	now := wsproto.NowMillis()
	startDurationMs := v.startBuffer.totalMs
	v.segmentStartMs = now - int64(startDurationMs)
	if v.segmentStartMs < 0 {
		v.segmentStartMs = now
	}
	v.segmentDurationMs = v.startBuffer.totalMs
	v.state = vadStateSpeaking
	v.speechActivity = v.startActivity.Clone()
	packets := v.preRoll.Packets(trackID)
	packets = append(packets, v.startBuffer.Packets(trackID)...)
	v.preRoll.Reset()
	v.startBuffer.Reset()
	v.startActivity.Reset()
	return Result{
		Packets: packets,
		Events: []protocol.Event{{
			Event:     compat.EventSpeaking,
			TrackID:   trackID,
			Timestamp: now,
		}},
	}
}

func (v *VAD) confirmEndLocked(trackID string) Result {
	confirmed := true
	if v.mode == vadoutbound.TypeHybrid && v.scorer != nil {
		score, err := v.scoreLocked(v.tailBuffer.Bytes())
		if err == nil {
			confirmed = score < v.sileroEndThreshold
		}
	}
	if !confirmed {
		v.state = vadStateSpeaking
		packets := v.tailBuffer.Packets(trackID)
		v.segmentDurationMs += v.tailBuffer.totalMs
		v.tailBuffer.Reset()
		return Result{Packets: packets}
	}
	now := wsproto.NowMillis()
	packets := v.tailBuffer.PrefixPackets(trackID, v.tailPaddingMs)
	packets = append(packets, mediaentity.Packet{TrackID: trackID, Kind: mediaentity.PacketKindSegmentEnd})
	startTime := v.segmentStartMs
	if startTime == 0 {
		startTime = now
	}
	result := Result{
		Packets: packets,
		Events: []protocol.Event{
			{
				Event:     compat.EventSilence,
				TrackID:   trackID,
				Timestamp: now,
			},
			{
				Event:     compat.EventEOU,
				TrackID:   trackID,
				Timestamp: now,
				StartTime: wsproto.Int64(startTime),
				EndTime:   wsproto.Int64(now),
			},
		},
	}
	v.resetToIdleLocked()
	return result
}

func (v *VAD) maybeForceSegmentEndLocked(trackID string, result Result) Result {
	if v.maxSegmentMs <= 0 || v.segmentDurationMs < v.maxSegmentMs {
		return result
	}
	now := wsproto.NowMillis()
	startTime := v.segmentStartMs
	if startTime == 0 {
		startTime = now
	}
	result.Packets = append(result.Packets, mediaentity.Packet{TrackID: trackID, Kind: mediaentity.PacketKindSegmentEnd})
	result.Events = append(result.Events,
		protocol.Event{Event: compat.EventSilence, TrackID: trackID, Timestamp: now},
		protocol.Event{Event: compat.EventEOU, TrackID: trackID, Timestamp: now, StartTime: wsproto.Int64(startTime), EndTime: wsproto.Int64(now)},
	)
	v.resetToIdleLocked()
	return result
}

func (v *VAD) shouldEnterCandidateEndLocked() bool {
	if v.speechActivity.totalMs < v.endWindowMs {
		return false
	}
	return v.speechActivity.Ratio() <= v.endActivityRatio
}

func (v *VAD) shouldResumeSpeakingLocked() bool {
	if v.speechActivity.totalMs < v.endWindowMs {
		return false
	}
	return v.speechActivity.Ratio() >= v.resumeActivityRatio
}

func (v *VAD) shouldConfirmEndLocked() bool {
	if v.speechActivity.totalMs < v.endWindowMs {
		return false
	}
	return v.speechActivity.Ratio() <= v.endActivityRatio
}

func (v *VAD) enterHitLocked(rms float64) bool {
	return rms >= math.Max(v.minEnterRMS, v.noiseThresholdLocked(v.enterRatio))
}

func (v *VAD) keepHitLocked(rms float64) bool {
	return rms >= math.Max(v.minKeepRMS, v.noiseThresholdLocked(v.keepRatio))
}

func (v *VAD) noiseThresholdLocked(ratio float64) float64 {
	if !v.noiseReady {
		return 0
	}
	return v.noiseFloor * ratio
}

func (v *VAD) observeNoiseLocked(rms float64) {
	if !v.noiseReady {
		v.noiseFloor = rms
		v.noiseReady = true
		return
	}
	v.noiseFloor = v.noiseFloor*(1-defaultVADNoiseAlpha) + rms*defaultVADNoiseAlpha
}

func (v *VAD) startWindowBytesLocked() []byte {
	window := v.preRoll.Bytes()
	window = append(window, v.startBuffer.Bytes()...)
	return window
}

func (v *VAD) scoreLocked(payload []byte) (float32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), v.scoreTimeout)
	defer cancel()
	return v.scorer.Score(ctx, payload, v.sampleRate)
}

func (v *VAD) resetToIdleLocked() {
	v.state = vadStateIdle
	v.preRoll.Reset()
	v.startBuffer.Reset()
	v.tailBuffer.Reset()
	v.startActivity.Reset()
	v.speechActivity.Reset()
	v.segmentStartMs = 0
	v.segmentDurationMs = 0
}

func pcmDurationMs(payload []byte, sampleRate int) int {
	if sampleRate <= 0 || len(payload) < 2 {
		return 0
	}
	sampleCount := len(payload) / 2
	if sampleCount == 0 {
		return 0
	}
	return (sampleCount*1000 + sampleRate - 1) / sampleRate
}

func rmsPCM16LE(payload []byte) float64 {
	if len(payload) < 2 {
		return 0
	}
	var sum float64
	sampleCount := 0
	for i := 0; i+1 < len(payload); i += 2 {
		sample := int16(uint16(payload[i]) | uint16(payload[i+1])<<8)
		value := float64(sample)
		sum += value * value
		sampleCount++
	}
	if sampleCount == 0 {
		return 0
	}
	return math.Sqrt(sum / float64(sampleCount))
}

func derefVADString(option *wsproto.VADOption, pick func(*wsproto.VADOption) *string) string {
	if option == nil {
		return ""
	}
	v := pick(option)
	if v == nil {
		return ""
	}
	return *v
}

func valueOrUint32(option *wsproto.VADOption, pick func(*wsproto.VADOption) *uint32, fallback uint32) uint32 {
	if option == nil {
		return fallback
	}
	v := pick(option)
	if v == nil || *v == 0 {
		return fallback
	}
	return *v
}

func valueOrUint64(option *wsproto.VADOption, pick func(*wsproto.VADOption) *uint64, fallback uint64) uint64 {
	if option == nil {
		return fallback
	}
	v := pick(option)
	if v == nil || *v == 0 {
		return fallback
	}
	return *v
}

func valueOrFloat32(option *wsproto.VADOption, pick func(*wsproto.VADOption) *float32, fallback float32) float32 {
	if option == nil {
		return fallback
	}
	v := pick(option)
	if v == nil || *v <= 0 {
		return fallback
	}
	return *v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
