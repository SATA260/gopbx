package vad

import (
	"context"
	"fmt"
	"sync"
)

type MockScorer struct {
	mu     sync.Mutex
	Scores []float32
	Err    error
	Calls  int
}

func (m *MockScorer) Name() string { return "mock" }

func (m *MockScorer) Close() error { return nil }

func (m *MockScorer) Score(context.Context, []byte, int) (float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls++
	if m.Err != nil {
		return 0, m.Err
	}
	if len(m.Scores) == 0 {
		return 0, fmt.Errorf("mock scorer has no scores")
	}
	index := m.Calls - 1
	if index >= len(m.Scores) {
		index = len(m.Scores) - 1
	}
	return m.Scores[index], nil
}
