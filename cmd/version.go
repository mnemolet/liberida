package main

import (
	"fmt"

	"github.com/mnemolet/liberida/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(version.Info())
	},
}

var versionShortCmd = &cobra.Command{
	Use:   "version-short",
	Short: "Print short version string",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.Short())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(versionShortCmd)
}
