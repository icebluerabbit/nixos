package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// stubBody is a Body that records what it was told.
type stubBody struct {
	title    string
	inited   bool
	messages []tea.Msg
	width    int
}

func (b *stubBody) Title() string { return b.title }

func (b *stubBody) Init() tea.Cmd {
	b.inited = true
	return nil
}

func (b *stubBody) Update(msg tea.Msg) (Body, tea.Cmd) {
	b.messages = append(b.messages, msg)
	return b, nil
}

func (b *stubBody) View(width int) string {
	b.width = width
	return "body contents"
}

func newTestShell(cancel func()) (*Shell, *stubBody) {
	body := &stubBody{title: "flac-util collect"}
	s := NewShell(body, cancel)
	s.now = func() time.Time { return s.started.Add(90 * time.Second) }
	return s, body
}

func TestShellInitStartsTheBody(t *testing.T) {
	s, body := newTestShell(nil)

	s.Init()
	if !body.inited {
		t.Error("the body was not initialised")
	}
}

func TestShellTracksWindowSize(t *testing.T) {
	s, body := newTestShell(nil)

	s.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if s.width != 120 || s.height != 40 {
		t.Errorf("size = %dx%d, want 120x40", s.width, s.height)
	}

	s.View()
	if body.width != 120 {
		t.Errorf("the body was rendered at width %d, want 120", body.width)
	}
}

// TestShellQuitCancelsRatherThanKilling is the behaviour that keeps a run
// safe: asking to stop must wind the work down, not abandon it mid-write.
func TestShellQuitCancelsRatherThanKilling(t *testing.T) {
	var cancelled int
	s, _ := newTestShell(func() { cancelled++ })

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cancelled != 1 {
		t.Fatalf("cancel called %d times, want 1", cancelled)
	}
	if cmd != nil {
		t.Error("quitting issued a command, want the display to stay up until the work drains")
	}

	// Asking twice must not cancel twice.
	s.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cancelled != 1 {
		t.Errorf("cancel called %d times after a second request, want 1", cancelled)
	}
	if !s.stopping {
		t.Error("stopping = false after a quit request")
	}
}

func TestShellQuitWithoutCancelDoesNotPanic(t *testing.T) {
	s, _ := newTestShell(nil)
	s.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if !s.stopping {
		t.Error("stopping = false after a quit request")
	}
}

func TestShellHelpToggle(t *testing.T) {
	s, _ := newTestShell(nil)
	before := s.help.ShowAll

	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if s.help.ShowAll == before {
		t.Error("the help did not toggle")
	}

	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if s.help.ShowAll != before {
		t.Error("the help did not toggle back")
	}
}

func TestShellFinishedQuits(t *testing.T) {
	s, _ := newTestShell(nil)

	_, cmd := s.Update(FinishedMsg{})
	if cmd == nil {
		t.Fatal("FinishedMsg issued no command, want a quit")
	}
	if !s.finished {
		t.Error("finished = false after FinishedMsg")
	}
	if s.Err() != nil {
		t.Errorf("Err() = %v, want nil", s.Err())
	}
}

func TestShellFinishedCarriesTheError(t *testing.T) {
	s, _ := newTestShell(nil)
	want := errors.New("the run broke")

	s.Update(FinishedMsg{Err: want})
	if !errors.Is(s.Err(), want) {
		t.Errorf("Err() = %v, want %v", s.Err(), want)
	}
	if got := s.View(); !strings.Contains(got, "the run broke") {
		t.Errorf("view does not mention the error:\n%s", got)
	}
}

func TestShellForwardsUnhandledMessagesToTheBody(t *testing.T) {
	s, body := newTestShell(nil)

	s.Update(EventMsg{Path: "track.flac"})
	if len(body.messages) != 1 {
		t.Fatalf("the body received %d messages, want 1", len(body.messages))
	}
	if _, ok := body.messages[0].(EventMsg); !ok {
		t.Errorf("the body received %T, want EventMsg", body.messages[0])
	}
}

func TestShellViewIncludesTheChrome(t *testing.T) {
	s, _ := newTestShell(nil)
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	got := s.View()
	for _, want := range []string{"flac-util collect", "body contents", "1m30s", "stop"} {
		if !strings.Contains(got, want) {
			t.Errorf("view does not contain %q:\n%s", want, got)
		}
	}
}

func TestShellViewShowsStoppingState(t *testing.T) {
	s, _ := newTestShell(func() {})
	s.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if got := s.View(); !strings.Contains(got, "stopping") {
		t.Errorf("view does not report that the run is stopping:\n%s", got)
	}
}

func TestShellViewShowsFinishedState(t *testing.T) {
	s, _ := newTestShell(nil)
	s.Update(FinishedMsg{})

	if got := s.View(); !strings.Contains(got, "finished") {
		t.Errorf("view does not report that the run finished:\n%s", got)
	}
}

// TestShellViewSurvivesANarrowTerminal covers the arithmetic in the header,
// which subtracts widths and could otherwise ask for a negative repeat count.
func TestShellViewSurvivesANarrowTerminal(t *testing.T) {
	s, _ := newTestShell(nil)
	s.Update(tea.WindowSizeMsg{Width: 4, Height: 4})

	if got := s.View(); got == "" {
		t.Error("view is empty at a narrow width")
	}
}
