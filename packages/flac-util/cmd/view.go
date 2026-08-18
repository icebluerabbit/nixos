package cmd

import (
	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"flac-util/internal/inventory"
	"flac-util/internal/report"
	"flac-util/internal/tui"
)

// viewFlags holds everything the view command accepts.
type viewFlags struct {
	extensions    []string
	reportSuffix  string
	workers       int
	cutoffFloorDB float64
}

// newViewCommand builds the view subcommand.
func newViewCommand(global *GlobalOptions) *cobra.Command {
	flags := &viewFlags{}

	cmd := &cobra.Command{
		Use:   "view [path...]",
		Short: "List the tracks under the given paths beside what was measured",
		Long: "Reads the reports collect left beside each track and lists them,\n" +
			"one table per release.\n\n" +
			"Nothing is written and nothing is judged. The cutoff column is\n" +
			"coloured by which row of interviewfor.red's table the measured\n" +
			"figure landed on, which is a shorthand for looking it up, not a\n" +
			"verdict on the track.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runView(cmd, args, global, flags)
		},
	}

	fs := cmd.Flags()
	fs.StringSliceVar(&flags.extensions, "extensions", []string{inventory.DefaultExtension}, "file extensions to list")
	fs.StringVar(&flags.reportSuffix, "report-suffix", report.DefaultReportSuffix, "suffix of the report written beside each track")
	fs.IntVar(&flags.workers, "workers", 0, "reports read at once; zero selects one per CPU")
	fs.Float64Var(&flags.cutoffFloorDB, "cutoff-floor-db", inventory.DefaultCutoffFloorDB,
		"how far below the loudest bin still counts as content when reading a cutoff")

	return cmd
}

// runView scans the given paths and shows what was found.
func runView(cmd *cobra.Command, roots []string, global *GlobalOptions, flags *viewFlags) error {
	groups, err := inventory.Scan(inventory.ScanOptions{
		Roots:        roots,
		Extensions:   flags.extensions,
		ReportSuffix: flags.reportSuffix,
		Workers:      flags.workers,
	})
	if err != nil {
		return err
	}

	opts := inventory.Options{CutoffFloorDB: flags.cutoffFloorDB}
	tables := make([]*inventory.Table, 0, len(groups))
	for _, group := range groups {
		tables = append(tables, inventory.NewTable(group, opts))
	}

	terminal := stdoutIsTerminal()
	interactive, err := global.UseInteractive(terminal)
	if err != nil {
		return err
	}
	if !interactive {
		return tui.PrintTables(cmd.OutOrStdout(), tables, terminal)
	}

	body := tui.NewViewModel(tables)
	shell := tui.NewShell(body, nil)
	// Reading takes the whole window and gives the terminal back untouched;
	// a collect run scrolls past and should stay in the scrollback.
	shell.AltScreen = true

	if _, err := tea.NewProgram(shell).Run(); err != nil {
		return err
	}
	return shell.Err()
}
