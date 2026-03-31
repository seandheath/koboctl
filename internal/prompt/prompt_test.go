package prompt_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/seandheath/koboctl/internal/prompt"
)

func newTestPrompter(input string) (*prompt.Prompter, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return prompt.NewPrompter(strings.NewReader(input), out), out
}

// -- Bool ------------------------------------------------------------------

func TestBool_YesInputs(t *testing.T) {
	for _, input := range []string{"y\n", "Y\n", "yes\n", "YES\n"} {
		p, _ := newTestPrompter(input)
		got, err := p.Bool("question", false)
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", input, err)
		}
		if !got {
			t.Errorf("input %q: expected true", input)
		}
	}
}

func TestBool_NoInputs(t *testing.T) {
	for _, input := range []string{"n\n", "N\n", "no\n", "NO\n"} {
		p, _ := newTestPrompter(input)
		got, err := p.Bool("question", true)
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", input, err)
		}
		if got {
			t.Errorf("input %q: expected false", input)
		}
	}
}

func TestBool_BareEnterDefaultYes(t *testing.T) {
	p, _ := newTestPrompter("\n")
	got, _ := p.Bool("question", true)
	if !got {
		t.Error("bare Enter with defaultYes=true should return true")
	}
}

func TestBool_BareEnterDefaultNo(t *testing.T) {
	p, _ := newTestPrompter("\n")
	got, _ := p.Bool("question", false)
	if got {
		t.Error("bare Enter with defaultYes=false should return false")
	}
}

func TestBool_EOFReturnsDefault(t *testing.T) {
	p, _ := newTestPrompter("") // empty reader → immediate EOF
	got, _ := p.Bool("question", true)
	if !got {
		t.Error("EOF should return default (true)")
	}
	p2, _ := newTestPrompter("")
	got2, _ := p2.Bool("question", false)
	if got2 {
		t.Error("EOF should return default (false)")
	}
}

func TestBool_InvalidThenValid(t *testing.T) {
	// First input is invalid, second is valid.
	p, out := newTestPrompter("maybe\ny\n")
	got, err := p.Bool("question", false)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true after re-prompt")
	}
	if !strings.Contains(out.String(), "Please enter y or n") {
		t.Error("expected re-prompt message")
	}
}

// -- String ----------------------------------------------------------------

func TestString_ValueEntered(t *testing.T) {
	p, _ := newTestPrompter("myvalue\n")
	got, err := p.String("question", "default")
	if err != nil {
		t.Fatal(err)
	}
	if got != "myvalue" {
		t.Errorf("got %q, want %q", got, "myvalue")
	}
}

func TestString_BareEnterReturnsDefault(t *testing.T) {
	p, _ := newTestPrompter("\n")
	got, _ := p.String("question", "thedefault")
	if got != "thedefault" {
		t.Errorf("got %q, want %q", got, "thedefault")
	}
}

func TestString_EOFReturnsDefault(t *testing.T) {
	p, _ := newTestPrompter("")
	got, _ := p.String("question", "fallback")
	if got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

// -- Choice ----------------------------------------------------------------

func TestChoice_ValidInput(t *testing.T) {
	p, _ := newTestPrompter("nightly\n")
	got, err := p.Choice("channel", []string{"stable", "nightly"}, "stable")
	if err != nil {
		t.Fatal(err)
	}
	if got != "nightly" {
		t.Errorf("got %q, want %q", got, "nightly")
	}
}

func TestChoice_CaseInsensitive(t *testing.T) {
	p, _ := newTestPrompter("STABLE\n")
	got, _ := p.Choice("channel", []string{"stable", "nightly"}, "nightly")
	if got != "stable" {
		t.Errorf("got %q, want stable", got)
	}
}

func TestChoice_BareEnterReturnsDefault(t *testing.T) {
	p, _ := newTestPrompter("\n")
	got, _ := p.Choice("channel", []string{"stable", "nightly"}, "stable")
	if got != "stable" {
		t.Errorf("got %q, want stable", got)
	}
}

func TestChoice_InvalidThenValid(t *testing.T) {
	p, out := newTestPrompter("weekly\nstable\n")
	got, err := p.Choice("channel", []string{"stable", "nightly"}, "stable")
	if err != nil {
		t.Fatal(err)
	}
	if got != "stable" {
		t.Errorf("got %q, want stable", got)
	}
	if !strings.Contains(out.String(), "Invalid choice") {
		t.Error("expected invalid choice message")
	}
}

// -- StringList ------------------------------------------------------------

func TestStringList_CommaSeparated(t *testing.T) {
	p, _ := newTestPrompter("1.1.1.1,8.8.8.8\n")
	got, err := p.StringList("DNS servers", []string{"default"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "1.1.1.1" || got[1] != "8.8.8.8" {
		t.Errorf("got %v, want [1.1.1.1 8.8.8.8]", got)
	}
}

func TestStringList_BareEnterReturnsDefault(t *testing.T) {
	defaults := []string{"185.228.168.168", "185.228.169.168"}
	p, _ := newTestPrompter("\n")
	got, _ := p.StringList("DNS servers", defaults)
	if len(got) != 2 || got[0] != defaults[0] {
		t.Errorf("got %v, want %v", got, defaults)
	}
}

func TestStringList_TrimsWhitespace(t *testing.T) {
	p, _ := newTestPrompter(" 1.1.1.1 , 8.8.8.8 \n")
	got, _ := p.StringList("DNS", nil)
	if len(got) != 2 || got[0] != "1.1.1.1" || got[1] != "8.8.8.8" {
		t.Errorf("got %v, want [1.1.1.1 8.8.8.8]", got)
	}
}
