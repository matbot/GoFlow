package cmd

import (
	"fmt"
	"github.com/matbot/goflow/workflow"
	"github.com/spf13/cobra"
	"os"
)

var runCmd = &cobra.Command{
	Use:   "run [workflow file]",
	Short: "Run a workflow",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		workflowFile := args[0]
		err := workflow.ExecuteWorkflow(workflowFile)
		if err != nil {
			fmt.Printf("Error executing workflow: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
