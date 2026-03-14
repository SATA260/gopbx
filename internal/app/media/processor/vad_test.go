package processor

import (
	"errors"
	"testing"

	vadoutbound "gopbx/internal/adapter/outbound/vad"
	mediaentity "gopbx/internal/domain/media"
	"gopbx/pkg/wsproto"
)

func TestVADHybridEmitsSpeakingAndEOU(t *testing.T) {
	scorer := &vadoutbound.MockScorer{Scores: []float32{0.95, 0.05}}
	vad := NewVAD(&wsproto.VADOption{
		Type:           stringPtr("hybrid"),
		Samplerate:     uint32Ptr(16000),
		SpeechPadding:  uint64Ptr(20),
		SilencePadding: uint64Ptr(60),
		Ratio:          float32Ptr(1.5),
		VoiceThreshold: float32Ptr(0.6),
	}, scorer)

	if got := vad.Process(audioPacket("track-1", silentFrame())); len(got.Events) != 0 || len(got.Packets) != 0 {
		t.Fatalf("expected silence pre-roll to be buffered only, got %+v", got)
	}
	if got := vad.Process(audioPacket("track-1", voiceFrame(1400))); len(got.Events) != 0 || len(got.Packets) != 0 {
		t.Fatalf("expected first voiced frame to stay in candidateStart, got %+v", got)
	}
	if got := vad.Process(audioPacket("track-1", voiceFrame(1400))); len(got.Events) != 0 || len(got.Packets) != 0 {
		t.Fatalf("expected second voiced frame to still wait for sliding ratio window, got %+v", got)
	}
	start := vad.Process(audioPacket("track-1", voiceFrame(1400)))
	if len(start.Events) != 1 || start.Events[0].Event != "speaking" {
		t.Fatalf("expected speaking event on start confirm, got %+v", start.Events)
	}
	if len(start.Packets) != 4 {
		t.Fatalf("expected pre-roll plus 3 voiced packets, got %d", len(start.Packets))
	}
	if start.Packets[0].ResolvedKind() != mediaentity.PacketKindAudio {
		t.Fatalf("expected voiced packets to be audio, got %+v", start.Packets[0])
	}

	if got := vad.Process(audioPacket("track-1", silentFrame())); len(got.Events) != 0 || len(got.Packets) != 1 {
		t.Fatalf("expected first trailing silence to still pass through before ratio drops, got %+v", got)
	}
	if got := vad.Process(audioPacket("track-1", silentFrame())); len(got.Events) != 0 || len(got.Packets) != 1 {
		t.Fatalf("expected second trailing silence to still pass through before entering end state, got %+v", got)
	}
	if got := vad.Process(audioPacket("track-1", silentFrame())); len(got.Events) != 0 || len(got.Packets) != 0 {
		t.Fatalf("expected third trailing silence to enter candidateEnd without finalizing, got %+v", got)
	}
	if got := vad.Process(audioPacket("track-1", silentFrame())); len(got.Events) != 0 || len(got.Packets) != 0 {
		t.Fatalf("expected fourth trailing silence to keep buffering tail, got %+v", got)
	}
	end := vad.Process(audioPacket("track-1", silentFrame()))
	if len(end.Events) != 2 {
		t.Fatalf("expected silence and eou events, got %+v", end.Events)
	}
	if end.Events[0].Event != "silence" || end.Events[1].Event != "eou" {
		t.Fatalf("unexpected end events: %+v", end.Events)
	}
	if len(end.Packets) == 0 {
		t.Fatal("expected tail packets and segmentEnd marker")
	}
	last := end.Packets[len(end.Packets)-1]
	if last.ResolvedKind() != mediaentity.PacketKindSegmentEnd {
		t.Fatalf("expected last packet to be segmentEnd, got %+v", last)
	}
	if scorer.Calls != 2 {
		t.Fatalf("expected scorer to be called for start and end, got %d", scorer.Calls)
	}
}

func TestVADHybridRejectsFalseStart(t *testing.T) {
	scorer := &vadoutbound.MockScorer{Scores: []float32{0.2, 0.95}}
	vad := NewVAD(&wsproto.VADOption{
		Type:           stringPtr("hybrid"),
		Samplerate:     uint32Ptr(16000),
		SpeechPadding:  uint64Ptr(20),
		SilencePadding: uint64Ptr(60),
		Ratio:          float32Ptr(1.5),
		VoiceThreshold: float32Ptr(0.6),
	}, scorer)

	vad.Process(audioPacket("track-1", voiceFrame(1400)))
	vad.Process(audioPacket("track-1", voiceFrame(1400)))
	rejected := vad.Process(audioPacket("track-1", voiceFrame(1400)))
	if len(rejected.Events) != 0 || len(rejected.Packets) != 0 {
		t.Fatalf("expected rejected start to emit nothing, got %+v", rejected)
	}
	vad.Process(audioPacket("track-1", voiceFrame(1400)))
	vad.Process(audioPacket("track-1", voiceFrame(1400)))
	accepted := vad.Process(audioPacket("track-1", voiceFrame(1400)))
	if len(accepted.Events) != 1 || accepted.Events[0].Event != "speaking" {
		t.Fatalf("expected second attempt to pass, got %+v", accepted.Events)
	}
	if scorer.Calls != 2 {
		t.Fatalf("expected two scorer calls, got %d", scorer.Calls)
	}
}

func TestVADHybridFallsBackToEnergyOnScorerError(t *testing.T) {
	vad := NewVAD(&wsproto.VADOption{
		Type:           stringPtr("hybrid"),
		Samplerate:     uint32Ptr(16000),
		SpeechPadding:  uint64Ptr(20),
		SilencePadding: uint64Ptr(60),
		Ratio:          float32Ptr(1.5),
		VoiceThreshold: float32Ptr(0.6),
	}, &vadoutbound.MockScorer{Err: errors.New("sidecar unavailable")})

	vad.Process(audioPacket("track-1", voiceFrame(1400)))
	vad.Process(audioPacket("track-1", voiceFrame(1400)))
	got := vad.Process(audioPacket("track-1", voiceFrame(1400)))
	if len(got.Events) != 1 || got.Events[0].Event != "speaking" {
		t.Fatalf("expected energy fallback to confirm start, got %+v", got.Events)
	}
}

func TestVADEnergyUsesSlidingActivityRatio(t *testing.T) {
	vad := NewVAD(&wsproto.VADOption{
		Type:           stringPtr("energy"),
		Samplerate:     uint32Ptr(16000),
		SpeechPadding:  uint64Ptr(20),
		SilencePadding: uint64Ptr(60),
		Ratio:          float32Ptr(1.5),
	}, nil)

	if got := vad.Process(audioPacket("track-1", voiceFrame(1400))); len(got.Events) != 0 {
		t.Fatalf("expected first frame to only seed the sliding window, got %+v", got)
	}
	if got := vad.Process(audioPacket("track-1", silentFrame())); len(got.Events) != 0 {
		t.Fatalf("expected mixed activity below full window to emit nothing, got %+v", got)
	}
	start := vad.Process(audioPacket("track-1", voiceFrame(1400)))
	if len(start.Events) != 1 || start.Events[0].Event != "speaking" {
		t.Fatalf("expected 2/3 active frames in sliding window to confirm speech, got %+v", start.Events)
	}

	for i := 0; i < 4; i++ {
		if got := vad.Process(audioPacket("track-1", silentFrame())); len(got.Events) != 0 {
			t.Fatalf("expected no end before activity ratio fully drops, frame=%d got=%+v", i, got.Events)
		}
	}
	end := vad.Process(audioPacket("track-1", silentFrame()))
	if len(end.Events) != 2 || end.Events[0].Event != "silence" || end.Events[1].Event != "eou" {
		t.Fatalf("expected low activity ratio to end utterance, got %+v", end.Events)
	}
}

func audioPacket(trackID string, data []byte) mediaentity.Packet {
	return mediaentity.Packet{TrackID: trackID, Data: data, Kind: mediaentity.PacketKindAudio}
}

func silentFrame() []byte {
	return make([]byte, 640)
}

func voiceFrame(amplitude int16) []byte {
	frame := make([]byte, 640)
	for i := 0; i+1 < len(frame); i += 2 {
		frame[i] = byte(amplitude)
		frame[i+1] = byte(uint16(amplitude) >> 8)
	}
	return frame
}

func stringPtr(v string) *string { return &v }

func uint32Ptr(v uint32) *uint32 { return &v }

func uint64Ptr(v uint64) *uint64 { return &v }

func float32Ptr(v float32) *float32 { return &v }
