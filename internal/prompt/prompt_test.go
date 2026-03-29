package prompt

import (
	"bufio"
	"strings"
	"testing"
)

func newTestPrompter(input string) *TTYPrompter {
	return New(bufio.NewReader(strings.NewReader(input)))
}

func TestNoopPrompter_Text(t *testing.T) {
	p := NoopPrompter{}
	got, err := p.Text("Name", "default-val")
	if err != nil {
		t.Fatal(err)
	}
	if got != "default-val" {
		t.Errorf("got %q, want %q", got, "default-val")
	}
}

func TestNoopPrompter_Confirm(t *testing.T) {
	p := NoopPrompter{}

	got, err := p.Confirm("Continue?", true)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("got false, want true")
	}

	got, err = p.Confirm("Continue?", false)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("got true, want false")
	}
}

func TestTTYPrompter_Text_WithInput(t *testing.T) {
	p := newTestPrompter("my-value\n")
	got, err := p.Text("Name", "default")
	if err != nil {
		t.Fatal(err)
	}
	if got != "my-value" {
		t.Errorf("got %q, want %q", got, "my-value")
	}
}

func TestTTYPrompter_Text_EmptyReturnsDefault(t *testing.T) {
	p := newTestPrompter("\n")
	got, err := p.Text("Name", "default")
	if err != nil {
		t.Fatal(err)
	}
	if got != "default" {
		t.Errorf("got %q, want %q", got, "default")
	}
}

func TestTTYPrompter_Text_WhitespaceTrimmed(t *testing.T) {
	p := newTestPrompter("  hello  \n")
	got, err := p.Text("Name", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestTTYPrompter_Confirm(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultVal bool
		want       bool
	}{
		{"yes", "y\n", false, true},
		{"YES", "YES\n", false, true},
		{"no", "n\n", true, false},
		{"NO", "NO\n", true, false},
		{"empty default true", "\n", true, true},
		{"empty default false", "\n", false, false},
		{"garbage default true", "maybe\n", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestPrompter(tt.input)
			got, err := p.Confirm("Continue?", tt.defaultVal)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
