package tui

import (
	"errors"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// stubBody is a Body that records what it was told.
type stubBody struct {
	title    string
	inited   bool
	messages []tea.Msg
	keys     []key.Binding
	status   string
	width    int
}

func (b *stubBody) Title() string { return b.title }

func (b *stubBody) Keys() []key.Binding { return b.keys }

func (b *stubBody) Status() string { return b.status }

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
	body := &stubBody{title: "collect", status: "1m30s"}
	s := NewShell(body, cancel)
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

	s.render()
	if body.width != 120 {
		t.Errorf("the body was rendered at width %d, want 120", body.width)
	}
}

// TestShellQuitCancelsRatherThanKilling is the behaviour that keeps a run
// safe: asking to stop must wind the work down, not abandon it mid-write.
func TestShellQuitCancelsRatherThanKilling(t *testing.T) {
	var cancelled int
	s, _ := newTestShell(func() { cancelled++ })

	_, cmd := s.Update(keyPress("q"))
	if cancelled != 1 {
		t.Fatalf("cancel called %d times, want 1", cancelled)
	}
	if cmd != nil {
		t.Error("quitting issued a command, want the display to stay up until the work drains")
	}

	// Asking twice must not cancel twice.
	s.Update(keyPress("ctrl+c"))
	if cancelled != 1 {
		t.Errorf("cancel called %d times after a second request, want 1", cancelled)
	}
	if !s.stopping {
		t.Error("stopping = false after a quit request")
	}
}

// TestShellQuitWithNoWorkBehindItExitsAtOnce covers the display that has
// nothing running: no work means nothing will ever send a FinishedMsg, so a
// quit that only waited would leave no way out of the program.
func TestShellQuitWithNoWorkBehindItExitsAtOnce(t *testing.T) {
	for _, key := range []string{"q", "ctrl+c", "esc"} {
		t.Run(key, func(t *testing.T) {
			s, _ := newTestShell(nil)

			_, cmd := s.Update(keyPress(key))
			if cmd == nil {
				t.Fatalf("%q issued no command, want a quit", key)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Errorf("%q issued %T, want a quit", key, cmd())
			}
			if s.stopping {
				t.Error("stopping = true, want the display to have gone rather than waited")
			}
		})
	}
}

func TestShellHelpToggle(t *testing.T) {
	s, _ := newTestShell(nil)
	before := s.help.ShowAll

	s.Update(keyPress("?"))
	if s.help.ShowAll == before {
		t.Error("the help did not toggle")
	}

	s.Update(keyPress("?"))
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

func TestShellFinishedReachesTheBody(t *testing.T) {
	s, body := newTestShell(nil)

	s.Update(FinishedMsg{})
	if len(body.messages) != 1 {
		t.Fatalf("the body received %d messages, want the FinishedMsg", len(body.messages))
	}
	if _, ok := body.messages[0].(FinishedMsg); !ok {
		t.Errorf("the body received %T, want FinishedMsg", body.messages[0])
	}
}

func TestShellFinishedCarriesTheError(t *testing.T) {
	s, _ := newTestShell(nil)
	want := errors.New("the run broke")

	s.Update(FinishedMsg{Err: want})
	if !errors.Is(s.Err(), want) {
		t.Errorf("Err() = %v, want %v", s.Err(), want)
	}
	if got := plain(s.render()); !strings.Contains(got, "the run broke") {
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

	got := plain(s.render())
	for _, want := range []string{strings.ToUpper(Name), "collect", "body contents", "1m30s", "stop"} {
		if !strings.Contains(got, want) {
			t.Errorf("view does not contain %q:\n%s", want, got)
		}
	}
}

func TestShellViewShowsStoppingState(t *testing.T) {
	s, _ := newTestShell(func() {})
	s.Update(keyPress("ctrl+c"))

	if got := plain(s.render()); !strings.Contains(got, "stopping") {
		t.Errorf("view does not report that the run is stopping:\n%s", got)
	}
}

func TestShellViewShowsFinishedState(t *testing.T) {
	s, _ := newTestShell(nil)
	s.Update(FinishedMsg{})

	if got := plain(s.render()); !strings.Contains(got, "finished") {
		t.Errorf("view does not report that the run finished:\n%s", got)
	}
}

// TestShellViewSurvivesANarrowTerminal covers the arithmetic in the header,
// which subtracts widths and could otherwise ask for a negative repeat count.
func TestShellViewSurvivesANarrowTerminal(t *testing.T) {
	s, _ := newTestShell(nil)
	s.Update(tea.WindowSizeMsg{Width: 4, Height: 4})

	if got := s.render(); got == "" {
		t.Error("view is empty at a narrow width")
	}
}

// TestShellHelpShowsTheBodysKeys is what makes "?" mean anything: the shell
// only knows two keys, and a help line listing those alone tells a reader
// nothing about the display in front of them.
func TestShellHelpShowsTheBodysKeys(t *testing.T) {
	s, body := newTestShell(nil)
	body.keys = []key.Binding{
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next release")),
		key.NewBinding(key.WithKeys("1"), key.WithHelp("1-9", "sort by column")),
	}
	s.Update(tea.WindowSizeMsg{Width: 200, Height: 40})

	got := plain(s.render())
	for _, want := range []string{"next release", "sort by column", "keys", "stop"} {
		if !strings.Contains(got, want) {
			t.Errorf("the help line does not mention %q:\n%s", want, got)
		}
	}
}

func TestShellHelpWithoutABodysKeys(t *testing.T) {
	// A body that is only watched contributes nothing, and the help line has
	// to hold together without it.
	s, _ := newTestShell(nil)
	s.Update(tea.WindowSizeMsg{Width: 200, Height: 40})

	keys := s.helpKeys()
	if got := len(keys.ShortHelp()); got != 2 {
		t.Errorf("ShortHelp() has %d bindings, want the shell's 2", got)
	}
	if got := len(keys.FullHelp()); got != 1 {
		t.Errorf("FullHelp() has %d groups, want 1", got)
	}
}

func TestShellHeaderNamesTheProgramAndTheSubcommand(t *testing.T) {
	s, _ := newTestShell(nil)
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	header := plain(s.header())
	if !strings.Contains(header, strings.ToUpper(Name)) {
		t.Errorf("the header does not name the program:\n%s", header)
	}
	if !strings.Contains(header, "collect") {
		t.Errorf("the header does not name the subcommand:\n%s", header)
	}
	if got, want := strings.Count(header, "\n")+1, 1; got != want {
		t.Errorf("the header is %d lines, want %d", got, want)
	}
	// The bar has to run the full width of the display, or the background
	// stops part way across and reads as a stray highlight.
	if got := lipgloss.Width(header); got != s.width {
		t.Errorf("the title bar is %d wide against a display of %d", got, s.width)
	}
}

// TestShellHeaderShowsTheBodysStatus checks the right of the title bar belongs
// to the body. The shell used to show a clock of its own there, which for a
// display with nothing running counted the reader rather than any work - and,
// with nothing to redraw it, sat at zero while it did.
func TestShellHeaderShowsTheBodysStatus(t *testing.T) {
	s, body := newTestShell(nil)
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if got := plain(s.header()); !strings.Contains(got, "1m30s") {
		t.Errorf("the header does not show the body's status:\n%s", got)
	}

	body.status = ""
	if got := lipgloss.Width(s.header()); got != s.width {
		t.Errorf("without a status the bar is %d wide, want %d", got, s.width)
	}
}
