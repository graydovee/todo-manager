package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/graydovee/todolist/todo-cli/internal/client"
	"github.com/graydovee/todolist/todo-cli/internal/config"
	"github.com/graydovee/todolist/todo-cli/internal/output"
	"github.com/spf13/cobra"
)

type AppContext struct {
	Config *config.Config
	Client *client.Client
	Output output.Format
	NewClient func(baseURL, apiKey string) *client.Client
}

func NewRootCommand() *cobra.Command {
	var (
		baseURLFlag string
		apiKeyFlag  string
		outputFlag  string
		appCtx      AppContext
	)

	rootCmd := &cobra.Command{
		Use:   "todo-cli",
		Short: "CLI for Todo Manager API",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.LoaderOptions{
				BaseURLOverride: baseURLFlag,
				APIKeyOverride:  apiKeyFlag,
			})
			if err != nil {
				return &ExitError{Code: 2, Err: err}
			}
			appCtx.Config = cfg
			switch outputFlag {
			case string(output.FormatPretty):
				appCtx.Output = output.FormatPretty
			default:
				appCtx.Output = output.FormatJSON
			}
			cmd.Root().SetContext(context.WithValue(cmd.Root().Context(), appContextKey{}, &appCtx))
			if cmd.CommandPath() == "todo-cli config view" || cmd.CommandPath() == "todo-cli config validate" || cmd.CommandPath() == "todo-cli login" {
				return nil
			}
			if err := config.Validate(cfg); err != nil {
				return &ExitError{Code: 2, Err: err}
			}
			if appCtx.NewClient != nil {
				appCtx.Client = appCtx.NewClient(cfg.BaseURL, cfg.APIKey)
			} else {
				appCtx.Client = client.New(cfg.BaseURL, cfg.APIKey)
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().StringVar(&baseURLFlag, "base-url", "", "Todo Manager server base URL")
	rootCmd.PersistentFlags().StringVar(&apiKeyFlag, "api-key", "", "Todo Manager API key")
	rootCmd.PersistentFlags().StringVar(&outputFlag, "output", string(output.FormatJSON), "Output format: json|pretty")
	rootCmd.SetContext(context.WithValue(context.Background(), appContextKey{}, &appCtx))

	rootCmd.AddCommand(newConfigCommand())
	rootCmd.AddCommand(newLoginCommand())
	rootCmd.AddCommand(newTodosCommand())
	return rootCmd
}

type appContextKey struct{}

func getAppContext(cmd *cobra.Command) *AppContext {
	rootCtx := cmd.Root().Context().Value(appContextKey{})
	appCtx, _ := rootCtx.(*AppContext)
	return appCtx
}

func writeResult(cmd *cobra.Command, value any) error {
	appCtx := getAppContext(cmd)
	if appCtx == nil {
		return fmt.Errorf("application context is not initialized")
	}
	return output.Write(cmd.OutOrStdout(), appCtx.Output, value)
}

func writeError(cmd *cobra.Command, err error) error {
	appCtx := getAppContext(cmd)
	if apiErr, ok := err.(*client.APIError); ok {
		if appCtx != nil {
			if writeErr := output.Write(cmd.ErrOrStderr(), appCtx.Output, apiErr); writeErr == nil {
				return err
			}
		}
	}
	_, _ = fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
	return err
}

func commandContext(cmd *cobra.Command) context.Context {
	return cmd.Context()
}

func mustStringSliceFlag(cmd *cobra.Command, name string) []string {
	values, _ := cmd.Flags().GetStringSlice(name)
	return values
}

func configUserHomeDir() (string, error) {
	return os.UserHomeDir()
}
