//go:build silero_native && cgo && linux

package vad

/*
#cgo CFLAGS: -Wall -Werror -std=c99
#cgo LDFLAGS: -lonnxruntime

#include "native_bridge.h"
*/
import "C"

import (
	"context"
	"fmt"
	"sync"
	"unsafe"
)

const (
	nativeStateLen   = 2 * 1 * 128
	nativeContextLen = 64
)

type nativeScorer struct {
	mu sync.Mutex

	api         *C.OrtApi
	env         *C.OrtEnv
	sessionOpts *C.OrtSessionOptions
	session     *C.OrtSession
	memoryInfo  *C.OrtMemoryInfo
	cStrings    map[string]*C.char
	closed      bool
	modelPath   string
}

func newNativeScorer(cfg nativeConfig) (Scorer, error) {
	if cfg.ModelPath == "" {
		return nil, nil
	}
	s := &nativeScorer{cStrings: map[string]*C.char{}, modelPath: cfg.ModelPath}
	s.api = C.GoOrtGetApi()
	if s.api == nil {
		return nil, fmt.Errorf("failed to get onnxruntime api")
	}
	logID := C.CString("gopbx-vad")
	defer C.free(unsafe.Pointer(logID))
	if err := s.wrapStatus(C.GoOrtCreateEnv(s.api, C.ORT_LOGGING_LEVEL_WARNING, logID, &s.env), "create env"); err != nil {
		s.Close()
		return nil, err
	}
	if err := s.wrapStatus(C.GoOrtCreateSessionOptions(s.api, &s.sessionOpts), "create session options"); err != nil {
		s.Close()
		return nil, err
	}
	if err := s.wrapStatus(C.GoOrtSetIntraOpNumThreads(s.api, s.sessionOpts, 1), "set intra threads"); err != nil {
		s.Close()
		return nil, err
	}
	if err := s.wrapStatus(C.GoOrtSetInterOpNumThreads(s.api, s.sessionOpts, 1), "set inter threads"); err != nil {
		s.Close()
		return nil, err
	}
	if err := s.wrapStatus(C.GoOrtSetSessionGraphOptimizationLevel(s.api, s.sessionOpts, C.ORT_ENABLE_ALL), "set graph optimization"); err != nil {
		s.Close()
		return nil, err
	}
	s.cStrings["modelPath"] = C.CString(cfg.ModelPath)
	if err := s.wrapStatus(C.GoOrtCreateSession(s.api, s.env, s.cStrings["modelPath"], s.sessionOpts, &s.session), "create session"); err != nil {
		s.Close()
		return nil, err
	}
	if err := s.wrapStatus(C.GoOrtCreateCpuMemoryInfo(s.api, C.OrtArenaAllocator, C.OrtMemTypeDefault, &s.memoryInfo), "create cpu memory info"); err != nil {
		s.Close()
		return nil, err
	}
	s.cStrings["input"] = C.CString("input")
	s.cStrings["state"] = C.CString("state")
	s.cStrings["sr"] = C.CString("sr")
	s.cStrings["output"] = C.CString("output")
	s.cStrings["stateN"] = C.CString("stateN")
	return s, nil
}

func (s *nativeScorer) Name() string { return "silero-native" }

func (s *nativeScorer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.api != nil {
		C.GoOrtReleaseMemoryInfo(s.api, s.memoryInfo)
		C.GoOrtReleaseSession(s.api, s.session)
		C.GoOrtReleaseSessionOptions(s.api, s.sessionOpts)
		C.GoOrtReleaseEnv(s.api, s.env)
	}
	for _, ptr := range s.cStrings {
		C.free(unsafe.Pointer(ptr))
	}
	s.cStrings = nil
	s.memoryInfo = nil
	s.session = nil
	s.sessionOpts = nil
	s.env = nil
	return nil
}

func (s *nativeScorer) Score(ctx context.Context, pcm16le []byte, sampleRate int) (float32, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if sampleRate != 8000 && sampleRate != 16000 {
		return 0, fmt.Errorf("unsupported sample rate: %d", sampleRate)
	}
	samples, err := pcm16ToFloat32(pcm16le)
	if err != nil {
		return 0, err
	}
	if len(samples) == 0 {
		return 0, nil
	}
	windowSize := 512
	if sampleRate == 8000 {
		windowSize = 256
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, fmt.Errorf("native scorer is closed")
	}

	var maxProb float32
	var state [nativeStateLen]float32
	var ctxBuf [nativeContextLen]float32
	hasContext := false

	for offset := 0; offset < len(samples); offset += windowSize {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		chunk := make([]float32, windowSize)
		copy(chunk, samples[offset:minInt(offset+windowSize, len(samples))])
		input := chunk
		if hasContext {
			input = make([]float32, nativeContextLen+windowSize)
			copy(input, ctxBuf[:])
			copy(input[nativeContextLen:], chunk)
		}
		prob, err := s.runChunk(input, state[:], sampleRate)
		if err != nil {
			return 0, err
		}
		if prob > maxProb {
			maxProb = prob
		}
		copy(ctxBuf[:], chunk[len(chunk)-nativeContextLen:])
		hasContext = true
	}

	return maxProb, nil
}

func (s *nativeScorer) runChunk(input []float32, state []float32, sampleRate int) (float32, error) {
	var inputValue *C.OrtValue
	inputShape := []C.int64_t{1, C.int64_t(len(input))}
	if err := s.wrapStatus(C.GoOrtCreateTensorWithDataAsOrtValue(
		s.api,
		s.memoryInfo,
		unsafe.Pointer(&input[0]),
		C.size_t(len(input)*4),
		&inputShape[0],
		C.size_t(len(inputShape)),
		C.ONNX_TENSOR_ELEMENT_DATA_TYPE_FLOAT,
		&inputValue,
	), "create pcm tensor"); err != nil {
		return 0, err
	}
	defer C.GoOrtReleaseValue(s.api, inputValue)

	var stateValue *C.OrtValue
	stateShape := []C.int64_t{2, 1, 128}
	if err := s.wrapStatus(C.GoOrtCreateTensorWithDataAsOrtValue(
		s.api,
		s.memoryInfo,
		unsafe.Pointer(&state[0]),
		C.size_t(len(state)*4),
		&stateShape[0],
		C.size_t(len(stateShape)),
		C.ONNX_TENSOR_ELEMENT_DATA_TYPE_FLOAT,
		&stateValue,
	), "create state tensor"); err != nil {
		return 0, err
	}
	defer C.GoOrtReleaseValue(s.api, stateValue)

	rate := []C.int64_t{C.int64_t(sampleRate)}
	rateShape := []C.int64_t{1}
	var rateValue *C.OrtValue
	if err := s.wrapStatus(C.GoOrtCreateTensorWithDataAsOrtValue(
		s.api,
		s.memoryInfo,
		unsafe.Pointer(&rate[0]),
		C.size_t(8),
		&rateShape[0],
		C.size_t(len(rateShape)),
		C.ONNX_TENSOR_ELEMENT_DATA_TYPE_INT64,
		&rateValue,
	), "create sample-rate tensor"); err != nil {
		return 0, err
	}
	defer C.GoOrtReleaseValue(s.api, rateValue)

	inputNames := []*C.char{s.cStrings["input"], s.cStrings["state"], s.cStrings["sr"]}
	inputs := []*C.OrtValue{inputValue, stateValue, rateValue}
	outputNames := []*C.char{s.cStrings["output"], s.cStrings["stateN"]}
	outputs := []*C.OrtValue{nil, nil}
	if err := s.wrapStatus(C.GoOrtRun(
		s.api,
		s.session,
		nil,
		(**C.char)(unsafe.Pointer(&inputNames[0])),
		(**C.OrtValue)(unsafe.Pointer(&inputs[0])),
		C.size_t(len(inputNames)),
		(**C.char)(unsafe.Pointer(&outputNames[0])),
		C.size_t(len(outputNames)),
		(**C.OrtValue)(unsafe.Pointer(&outputs[0])),
	), "run session"); err != nil {
		return 0, err
	}
	defer C.GoOrtReleaseValue(s.api, outputs[0])
	defer C.GoOrtReleaseValue(s.api, outputs[1])

	var probPtr unsafe.Pointer
	if err := s.wrapStatus(C.GoOrtGetTensorMutableData(s.api, outputs[0], &probPtr), "get output tensor"); err != nil {
		return 0, err
	}
	var statePtr unsafe.Pointer
	if err := s.wrapStatus(C.GoOrtGetTensorMutableData(s.api, outputs[1], &statePtr), "get output state tensor"); err != nil {
		return 0, err
	}
	copy(state, unsafe.Slice((*float32)(statePtr), len(state)))
	return *(*float32)(probPtr), nil
}

func (s *nativeScorer) wrapStatus(status *C.OrtStatus, action string) error {
	if status == nil {
		return nil
	}
	message := C.GoString(C.GoOrtGetErrorMessage(s.api, status))
	C.GoOrtReleaseStatus(s.api, status)
	return fmt.Errorf("%s: %s", action, message)
}

func pcm16ToFloat32(payload []byte) ([]float32, error) {
	if len(payload)%2 != 0 {
		return nil, fmt.Errorf("pcm16le body length must be even")
	}
	out := make([]float32, 0, len(payload)/2)
	for i := 0; i+1 < len(payload); i += 2 {
		sample := int16(uint16(payload[i]) | uint16(payload[i+1])<<8)
		out = append(out, float32(sample)/32768.0)
	}
	return out, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
