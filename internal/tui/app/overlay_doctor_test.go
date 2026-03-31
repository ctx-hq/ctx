package app

import (
	"testing"
)

func TestDoctorOverlay_Toggle(t *testing.T) {
	svc := &mockService{}
	o := newDoctorOverlay(80, 24)

	if o.visible {
		t.Fatal("expected not visible initially")
	}

	cmd := o.Toggle(svc)
	if !o.visible {
		t.Fatal("expected visible after toggle")
	}
	if !o.loading {
		t.Fatal("expected loading=true after toggle on")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	o.Toggle(svc)
	if o.visible {
		t.Fatal("expected not visible after second toggle")
	}
}

func TestDoctorOverlay_LoadingState(t *testing.T) {
	o := newDoctorOverlay(80, 24)
	o.visible = true
	o.loading = true

	view := o.View()
	if view == "" {
		t.Fatal("expected non-empty view during loading")
	}
}
