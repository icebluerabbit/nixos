// Package tui draws a run while it happens.
//
// It is the only package that touches the terminal. Everything it displays
// arrives through the progress package, so the collector neither knows nor
// cares whether anybody is watching.
package tui

import "github.com/charmbracelet/lipgloss"

// Theme holds every style the display uses, so that no other file invents one.
type Theme struct {
	Title    lipgloss.Style
	Subtle   lipgloss.Style
	Label    lipgloss.Style
	Value    lipgloss.Style
	Good     lipgloss.Style
	Bad      lipgloss.Style
	Warn     lipgloss.Style
	Header   lipgloss.Style
	Footer   lipgloss.Style
	Selected lipgloss.Style
}

// Colours are given as adaptive pairs so the display stays legible on light
// and dark terminals alike.
var (
	colorTitle  = lipgloss.AdaptiveColor{Light: "#5A3E9B", Dark: "#C4B5FD"}
	colorSubtle = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	colorLabel  = lipgloss.AdaptiveColor{Light: "#374151", Dark: "#D1D5DB"}
	colorValue  = lipgloss.AdaptiveColor{Light: "#111827", Dark: "#F9FAFB"}
	colorGood   = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#6EE7B7"}
	colorBad    = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"}
	colorWarn   = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FCD34D"}
)

// NewTheme returns the default styles.
func NewTheme() Theme {
	return Theme{
		Title:    lipgloss.NewStyle().Foreground(colorTitle).Bold(true),
		Subtle:   lipgloss.NewStyle().Foreground(colorSubtle),
		Label:    lipgloss.NewStyle().Foreground(colorLabel),
		Value:    lipgloss.NewStyle().Foreground(colorValue).Bold(true),
		Good:     lipgloss.NewStyle().Foreground(colorGood),
		Bad:      lipgloss.NewStyle().Foreground(colorBad),
		Warn:     lipgloss.NewStyle().Foreground(colorWarn),
		Header:   lipgloss.NewStyle().Padding(0, 1),
		Footer:   lipgloss.NewStyle().Foreground(colorSubtle).Padding(0, 1),
		Selected: lipgloss.NewStyle().Foreground(colorValue).Bold(true),
	}
}
