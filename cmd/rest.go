package cmd

import (
	"log"

	"github.com/Bahaaio/pomo/config"
	"github.com/spf13/cobra"
)

var restCmd = &cobra.Command{
	Use:   "rest [duration]",
	Short: "Start a standalone rest session",
	Long: `Start a standalone rest timer session that is not part of a Pomodoro cycle.

Unlike "pomo break", rest sessions are not saved in the database and do not
start a work session when completed. Once the rest timer finishes, it sends a
notification and counts up the extra time elapsed until you quit.`,
	Example: `  pomo rest       # Start a rest session
  pomo rest 10m   # Start a 10 minute rest session`,

	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("restCmd args:", args)
		runTask(config.RestTask, cmd)
	},
}

func init() {
	rootCmd.AddCommand(restCmd)
}
