package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Body is the part of the display a subcommand supplies. The shell owns
// everything around it: the title, the elapsed clock, the key hints and the
// decision to quit.
type Body interface {
	// Title names what is being done, for the header.
	Title() string
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

	started  time.Time
	now      func() time.Time
	stopping bool
	finished bool
	err      error

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
		body:    body,
		theme:   NewTheme(),
		keys:    DefaultKeyMap(),
		help:    h,
		width:   80,
		height:  24,
		started: time.Now(),
		now:     time.Now,
		cancel:  cancel,
	}
}

// Init implements tea.Model.
func (s *Shell) Init() tea.Cmd { return s.body.Init() }

// Update implements tea.Model.
func (s *Shell) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, s.keys.Quit):
			// The first request asks the run to stop; the display stays up
			// until the work has actually drained.
			if !s.stopping {
				s.stopping = true
				if s.cancel != nil {
					s.cancel()
				}
			}
			return s, nil
		case key.Matches(msg, s.keys.Help):
			s.help.ShowAll = !s.help.ShowAll
			return s, nil
		}

	case FinishedMsg:
		s.finished = true
		s.err = msg.Err
		return s, tea.Quit
	}

	body, cmd := s.body.Update(msg)
	s.body = body
	return s, cmd
}

// View implements tea.Model.
func (s *Shell) View() string {
	var b strings.Builder

	b.WriteString(s.header())
	b.WriteString("\n\n")
	b.WriteString(s.body.View(s.width))
	b.WriteString("\n\n")
	b.WriteString(s.footer())
	b.WriteString("\n")
	return b.String()
}

// header renders the title line and elapsed clock.
func (s *Shell) header() string {
	elapsed := s.now().Sub(s.started).Round(time.Second)
	left := s.theme.Title.Render(s.body.Title())
	right := s.theme.Subtle.Render(elapsed.String())

	gap := s.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	return s.theme.Header.Render(left + strings.Repeat(" ", max(gap, 1)) + right)
}

// footer renders the status line and key hints.
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

	hints := s.help.View(s.keys)
	if status == "" {
		return s.theme.Footer.Render(hints)
	}
	return s.theme.Footer.Render(status + "  " + hints)
}

// Err returns the run's error once the display has finished.
func (s *Shell) Err() error { return s.err }
