package cmd

import (
	"log"

	"github.com/Bahaaio/pomo/config"
	"github.com/spf13/cobra"
)

var stopwatchCmd = &cobra.Command{
	Use:   "stopwatch",
	Short: "Start a stopwatch session",
	Long: `Start an open-ended count-up stopwatch work session.

Elapsed time is recorded as a work session in your statistics database
and displayed in the session summary when you quit.`,
	Example: `  pomo stopwatch                  # Start a stopwatch session
  pomo stopwatch -t "deep work"   # Start stopwatch session with custom title`,

	Args: cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("stopwatchCmd args:", args)
		runTask(config.StopwatchTask, cmd)
	},
}

func init() {
	stopwatchCmd.Flags().StringP(
		"title",
		"t",
		"",
		"stopwatch session title",
	)

	rootCmd.AddCommand(stopwatchCmd)
}
