package main

import (
	"fmt"
	"os"

	"github.com/mnemolet/liberida/internal/config"
	"github.com/mnemolet/liberida/internal/noninteractive"
	"github.com/mnemolet/liberida/internal/stdinutil"
	"github.com/mnemolet/liberida/internal/version"
	"github.com/spf13/cobra"
)

var (
	attachFiles []string
	quietMode   bool
)

var rootCmd = &cobra.Command{
	Use:   "liberida",
	Short: "LiberIda - local AI Agent",
	Long:  `CLI LiberIda AI agent that runs locally using Ollama.`,
	Version: version.Short(),
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		manager := config.NewManager()
		if err := manager.Load(); err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		cfg := manager.Get()

		noContext, _ := cmd.Flags().GetBool("no-context")
		if noContext {
			cfg.AutoContext = false
		}

		stdinData := detectNonInteractiveMode()

		contextStr, prompt, shouldRunNonInteractive := noninteractive.DetectInputModeWithArgs(stdinData, args)

		if !shouldRunNonInteractive {
			sessionID, _ := cmd.Flags().GetUint("session")
			forceNew, _ := cmd.Flags().GetBool("new")
			prov, err := createProvider(cfg)
			if err != nil {
				return err
			}
			return runChatSession(prov, cfg, sessionID, forceNew, attachFiles)
		}

		prov, err := createProvider(cfg)
		if err != nil {
			return err
		}

		runner := noninteractive.NewRunner(cfg, prov, quietMode)

		if len(attachFiles) > 0 {
			handler := noninteractive.NewAttachmentHandler()
			components, errs := handler.ProcessPaths(attachFiles)
			for _, e := range errs {
				if !quietMode {
					fmt.Fprintf(os.Stderr, "Warning: %v\n", e)
				}
			}
			if len(components) > 0 {
				attachStr := handler.FormatComponents(components)
				contextStr = contextStr + "\n\n" + attachStr
			}
		}

		return runner.Run(prompt, contextStr)
	},
}

func detectNonInteractiveMode() string {
	if !stdinutil.IsPiped() {
		return ""
	}

	data, err := stdinutil.ReadAllTrimmed()
	if err != nil {
		if !quietMode {
			fmt.Fprintf(os.Stderr, "Warning: failed to read stdin: %v\n", err)
		}
		return ""
	}

	return data
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().UintP("session", "s", 0, "Resume existing session by ID")
	rootCmd.PersistentFlags().Bool("new", false, "Force create new session (ignore --session)")
	rootCmd.PersistentFlags().Bool("no-context", false, "Disable automatic workspace context")
	rootCmd.PersistentFlags().StringArrayVarP(&attachFiles, "attach", "a", nil, "Attach files to the conversation (can be repeated or comma-separated)")
	rootCmd.PersistentFlags().BoolVarP(&quietMode, "quiet", "q", false, "Suppress all output except the raw LLM response")
}