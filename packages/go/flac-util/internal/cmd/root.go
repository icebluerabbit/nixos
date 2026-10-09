// Package cmd defines the command line interface.
//
// The commands here do nothing but turn flags into options and hand them to
// the packages that do the work. Keeping them that thin is what lets the rest
// of the program be tested without a terminal.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"homelab/flac-util/internal/report"
)

// Identity of the program, written into every report so that a file can be
// traced back to what produced it.
const (
	Name    = "flac-util"
	Version = "0.1.0"
)

// Display modes for the interactive interface.
const (
	// DisplayAuto uses the interactive display when the output is a terminal.
	DisplayAuto = "auto"
	// DisplayAlways forces it on.
	DisplayAlways = "always"
	// DisplayNever forces it off.
	DisplayNever = "never"
)

// DisplayModes returns the accepted values for the display flag.
func DisplayModes() []string { return []string{DisplayAuto, DisplayAlways, DisplayNever} }

// GlobalOptions are the settings every subcommand shares.
type GlobalOptions struct {
	// Display selects the interactive interface.
	Display string
	// Verbose logs each file rather than only the ones that went wrong.
	Verbose bool
}

// Tool returns the identity written into reports.
func (GlobalOptions) Tool() report.Tool {
	return report.Tool{Name: Name, Version: Version}
}

// UseInteractive reports whether the interactive display should be used.
//
// The default is to use it only when standard output is a terminal. Anywhere
// else - a pipe, a log file, a timer unit - its escape codes would be noise,
// and a run of this length is quite likely to happen in exactly those places.
func (o GlobalOptions) UseInteractive(isTerminal bool) (bool, error) {
	switch o.Display {
	case DisplayAlways:
		return true, nil
	case DisplayNever:
		return false, nil
	case DisplayAuto, "":
		return isTerminal, nil
	default:
		return false, fmt.Errorf("unknown display mode %q, want one of %v", o.Display, DisplayModes())
	}
}

// stdoutIsTerminal reports whether standard output is attached to a terminal.
func stdoutIsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// NewRootCommand builds the command tree.
func NewRootCommand() *cobra.Command {
	var global GlobalOptions

	root := &cobra.Command{
		Use:   Name,
		Short: "Measure FLAC files and record what was found",
		Long: "Measures FLAC files and writes what it found beside each one.\n\n" +
			"It reports observations and never conclusions: no file is judged\n" +
			"genuine or otherwise, because deciding that is the reader's job.\n" +
			"Audio files are opened read-only and are never modified.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&global.Display, "display", DisplayAuto,
		fmt.Sprintf("interactive display: one of %v", DisplayModes()))
	root.PersistentFlags().BoolVarP(&global.Verbose, "verbose", "v", false,
		"log every file, not only the ones that failed")

	root.AddCommand(newCollectCommand(&global))
	root.AddCommand(newViewCommand(&global))
	return root
}

// Execute runs the command tree and returns the process exit code.
func Execute() int {
	root := NewRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", Name, err)
		return 1
	}
	return 0
}
