package cmd

import (
	"github.com/Bahaaio/pomo/ui/stats"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Args:  cobra.MaximumNArgs(0),
	Short: "Display Pomodoro statistics and productivity metrics",
	Long: `Display Pomodoro statistics, session history, and grouped views.

Navigate between views using Tab/Shift+Tab, ←/→ (h/l), or 1-4:
  1: Overview      - Duration ratio, streaks, weekly bar chart, and 4-month heatmap
  2: Sessions      - Saved sessions ordered by date and time
  3: By Title (-t) - Sessions grouped by task title, ordered by date and hour
  4: By Date       - Sessions grouped by date

Use ↑/↓ (k/j) to navigate saved data, Enter/Space to expand or collapse groups,
and d to delete a session or group (with confirmation).`,
	Run: func(cmd *cobra.Command, args []string) {
		m := stats.New()
		p := tea.NewProgram(m, tea.WithAltScreen())

		_, err := p.Run()
		if err != nil {
			die(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
