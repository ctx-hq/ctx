package inline

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestProgressModel_Transitions(t *testing.T) {
	fn := func(_ context.Context, _ func(float64)) error { return nil }
	m := newProgressModel("Loading", fn, nil)

	// Initial state
	if m.done {
		t.Fatal("expected not done initially")
	}
	if m.percent != 0 {
		t.Fatalf("expected percent 0, got %f", m.percent)
	}

	// Progress update
	var model tea.Model = m
	model, _ = model.Update(progressMsg{percent: 0.5})
	pm := model.(progressModel)
	if pm.percent != 0.5 {
		t.Fatalf("expected percent 0.5, got %f", pm.percent)
	}

	// Completion
	model, _ = model.Update(progressDoneMsg{err: nil})
	pm = model.(progressModel)
	if !pm.done {
		t.Fatal("expected done after completion")
	}
	if pm.err != nil {
		t.Fatalf("expected no error, got %v", pm.err)
	}
}

func TestProgressModel_ErrorPropagation(t *testing.T) {
	fn := func(_ context.Context, _ func(float64)) error { return nil }
	m := newProgressModel("Loading", fn, nil)

	var model tea.Model = m
	model, _ = model.Update(progressDoneMsg{err: context.Canceled})
	pm := model.(progressModel)
	if pm.err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", pm.err)
	}
}

func TestRenderBar(t *testing.T) {
	tests := []struct {
		pct  float64
		want string
	}{
		{0.0, "[....................]"},
		{0.5, "[##########..........]"},
		{1.0, "[####################]"},
	}
	for _, tt := range tests {
		got := renderBar(tt.pct, 20)
		if got != tt.want {
			t.Errorf("renderBar(%f, 20) = %q, want %q", tt.pct, got, tt.want)
		}
	}
}
