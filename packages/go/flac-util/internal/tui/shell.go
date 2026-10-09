package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-runewidth"
)

// Body is the part of the display a subcommand supplies. The shell owns
// everything around it: the title, the elapsed clock, the key hints and the
// decision to quit.
type Body interface {
	// Title names what is being done, for the header. It is the subcommand's
	// name alone: the shell puts the program's name in front of it.
	Title() string
	// Status is what the body wants shown at the right of the title bar, or
	// empty for nothing. A run puts its clock here; something being read has
	// no clock worth showing, and an unmoving one is worse than none.
	Status() string
	// Keys are the bindings this body answers to, for the help line. Nil is
	// fine for a body that only watches.
	Keys() []key.Binding
	// Init returns any command the body needs to start, such as a ticker.
	Init() tea.Cmd
	// Update handles a message and returns the updated body.
	Update(tea.Msg) (Body, tea.Cmd)
	// View renders the body to the given width.
	View(width int) string
}

// FinishedMsg tells the shell the work has ended and the display may close.
type FinishedMsg struct {
	// Err is the run's error, if it had one.
	Err error
}

// Shell wraps a body in the chrome every subcommand shares.
type Shell struct {
	body   Body
	theme  Theme
	keys   KeyMap
	help   help.Model
	width  int
	height int

	stopping bool
	finished bool
	err      error

	// AltScreen takes over the whole window. A run that scrolls its progress
	// past wants this off; something being read wants it on.
	AltScreen bool

	// cancel stops the work. Quitting asks the run to wind down rather than
	// killing it outright, so that no file is left half written.
	cancel func()
}

// NewShell returns a shell around the given body. Cancel is called when the
// user asks to stop, and may be nil.
func NewShell(body Body, cancel func()) *Shell {
	h := help.New()
	h.ShowAll = false

	return &Shell{
		body:   body,
		theme:  NewTheme(),
		keys:   DefaultKeyMap(),
		help:   h,
		width:  80,
		height: 24,
		cancel: cancel,
	}
}

// Init implements tea.Model.
func (s *Shell) Init() tea.Cmd { return s.body.Init() }

// Update implements tea.Model.
func (s *Shell) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.keys.Quit):
			// With no work behind it there is nothing to wind down and nothing
			// that will ever send a FinishedMsg, so quitting has to be
			// immediate. Waiting would leave the display up with no way out of
			// it short of killing the process.
			if s.cancel == nil {
				s.finished = true
				return s, tea.Quit
			}

			// The first request asks the run to stop; the display stays up
			// until the work has actually drained.
			if !s.stopping {
				s.stopping = true
				s.cancel()
			}
			return s, nil
		case key.Matches(msg, s.keys.Help):
			s.help.ShowAll = !s.help.ShowAll
			return s, nil
		}

	case FinishedMsg:
		s.finished = true
		s.err = msg.Err
		// Let the body take a last reading before the display closes, or the
		// final frame shows the counters as they were up to a tick ago - a
		// finished run reporting three of four files done.
		body, _ := s.body.Update(msg)
		s.body = body
		return s, tea.Quit
	}

	body, cmd := s.body.Update(msg)
	s.body = body
	return s, cmd
}

// View implements tea.Model.
//
// The alternate screen is asked for per frame rather than at start-up, which
// is how Bubble Tea takes it now: a view either wants the whole window or it
// wants to leave the scrollback alone, and that is a property of what is being
// drawn.
func (s *Shell) View() tea.View {
	view := tea.NewView(s.render())
	view.AltScreen = s.AltScreen
	return view
}

// render draws the chrome and the body into it.
func (s *Shell) render() string {
	var b strings.Builder

	b.WriteString(s.header())
	b.WriteString("\n")
	b.WriteString(s.body.View(s.width))
	b.WriteString("\n\n")
	b.WriteString(s.footer())
	return b.String()
}

// Name is the program's name, shown at the head of every display.
const Name = "flac-util"

// mark is the glyph the banner opens with.
const mark = "♪"

// header renders the banner: a gradient bar carrying the program, what it is
// doing, and whatever the body wants said about it.
//
// The whole width is painted, cell by cell, so the colour moves across the bar
// rather than sitting behind the words. Terminals that cannot manage the
// colours degrade to a solid band, which is why the text has to be legible
// against either end of the ramp rather than against a middle.
func (s *Shell) header() string {
	width := max(s.width, 1)

	left := " " + mark + "  " + strings.ToUpper(Name) + "  "
	middle := s.body.Title()
	right := s.body.Status()
	if right != "" {
		right += " "
	}

	// The name is set in the strongest part of the ramp and everything else
	// follows it, so the eye lands on the program before the detail.
	gap := width - runewidth.StringWidth(left) - runewidth.StringWidth(middle) - runewidth.StringWidth(right)
	text := left + middle + strings.Repeat(" ", max(gap, 1)) + right

	return s.theme.paintBanner(text, runewidth.StringWidth(left))
}

// rule draws a line the width of the display.
func (s *Shell) rule() string {
	return s.theme.Border.Render(strings.Repeat("─", max(s.width, 1)))
}

// footer renders a rule, then the status and the key hints.
func (s *Shell) footer() string {
	var status string
	switch {
	case s.err != nil:
		status = s.theme.Bad.Render(fmt.Sprintf("stopped: %v", s.err))
	case s.finished:
		status = s.theme.Good.Render("finished")
	case s.stopping:
		status = s.theme.Warn.Render("stopping, waiting for work in flight")
	}

	hints := s.help.View(s.helpKeys())
	if status != "" {
		hints = status + s.theme.Subtle.Render("  ·  ") + hints
	}
	return s.rule() + "\n" + s.theme.Footer.Render(hints)
}

// helpKeys puts the body's bindings in front of the shell's own, so the help
// line describes what the display actually responds to rather than only the
// two keys every display shares.
func (s *Shell) helpKeys() help.KeyMap {
	return bodyKeys{body: s.body.Keys(), shell: s.keys}
}

// bodyKeys presents a body's bindings and the shell's together.
type bodyKeys struct {
	body  []key.Binding
	shell KeyMap
}

// ShortHelp implements help.KeyMap.
func (b bodyKeys) ShortHelp() []key.Binding {
	return append(append([]key.Binding{}, b.body...), b.shell.Help, b.shell.Quit)
}

// FullHelp implements help.KeyMap.
func (b bodyKeys) FullHelp() [][]key.Binding {
	if len(b.body) == 0 {
		return [][]key.Binding{{b.shell.Help, b.shell.Quit}}
	}
	return [][]key.Binding{b.body, {b.shell.Help, b.shell.Quit}}
}

// Err returns the run's error once the display has finished.
func (s *Shell) Err() error { return s.err }
