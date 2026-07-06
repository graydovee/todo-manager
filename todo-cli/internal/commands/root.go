package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/graydovee/todo-manager/todo-cli/internal/client"
	"github.com/graydovee/todo-manager/todo-cli/internal/config"
	"github.com/graydovee/todo-manager/todo-cli/internal/output"
	"github.com/spf13/cobra"
)

type AppContext struct {
	Config        *config.Config
	EffectiveUser *config.UserEntry
	Client        *client.Client
	Output        output.Format
	NewClient     func(baseURL, apiKey string) *client.Client
}

func NewRootCommand() *cobra.Command {
	var (
		baseURLFlag string
		apiKeyFlag  string
		userFlag    string
		outputFlag  string
		appCtx      AppContext
	)

	rootCmd := &cobra.Command{
		Use:   "todo-cli",
		Short: "CLI for Todo Manager API",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.LoaderOptions{})
			if err != nil {
				return &ExitError{Code: 2, Err: err}
			}
			appCtx.Config = cfg

			if cfg.Migrated() {
				if werr := cfg.MigrationWriteErr(); werr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: migrated config in memory but could not persist (%v); continuing.\n", werr)
				} else {
					fmt.Fprintln(cmd.ErrOrStderr(), "Migrated config to multi-user format.")
				}
			}

			format, err := output.Parse(outputFlag)
			if err != nil {
				return &ExitError{Code: 2, Err: err}
			}
			appCtx.Output = format

			cmd.Root().SetContext(context.WithValue(cmd.Root().Context(), appContextKey{}, &appCtx))

			// Config subcommands and login manage users/config themselves and do
			// not need an effective user or HTTP client.
			path := cmd.CommandPath()
			if path == "todo-cli login" || strings.HasPrefix(path, "todo-cli config") {
				return nil
			}

			effective, err := resolveEffectiveUser(cmd, cfg)
			if err != nil {
				return &ExitError{Code: 2, Err: err}
			}
			if err := config.ValidateUser(effective); err != nil {
				return &ExitError{Code: 2, Err: err}
			}
			appCtx.EffectiveUser = &effective

			if appCtx.NewClient != nil {
				appCtx.Client = appCtx.NewClient(effective.BaseURL, effective.APIKey)
			} else {
				appCtx.Client = client.New(effective.BaseURL, effective.APIKey)
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().StringVar(&baseURLFlag, "base-url", "", "Todo Manager server base URL")
	rootCmd.PersistentFlags().StringVar(&apiKeyFlag, "api-key", "", "Todo Manager API key")
	rootCmd.PersistentFlags().StringVarP(&userFlag, "user", "u", "", "User profile to use (defaults to auth.default_user)")
	rootCmd.PersistentFlags().StringVarP(&outputFlag, "output", "o", string(output.FormatJSON), "Output format: yaml|json (pretty is a backward-compatible alias for json)")
	rootCmd.SetContext(context.WithValue(context.Background(), appContextKey{}, &appCtx))

	rootCmd.AddCommand(newConfigCommand())
	rootCmd.AddCommand(newLoginCommand())
	rootCmd.AddCommand(newTodosCommand())
	return rootCmd
}

// resolveEffectiveUser resolves the effective user from cfg and the -u flag,
// then applies env and flag overrides to a copy of that user. Precedence is
// flag > env > file.
func resolveEffectiveUser(cmd *cobra.Command, cfg *config.Config) (config.UserEntry, error) {
	userFlag, _ := cmd.Flags().GetString("user")
	u, err := cfg.ResolveUser(userFlag)
	if err != nil {
		return u, err
	}
	if v := strings.TrimSpace(os.Getenv(config.EnvAPIKey)); v != "" {
		u.APIKey = v
	}
	if v := strings.TrimSpace(os.Getenv(config.EnvBaseURL)); v != "" {
		if normalized, err := config.NormalizeBaseURL(v); err == nil {
			u.BaseURL = normalized
		}
	}
	if v, _ := cmd.Flags().GetString("api-key"); strings.TrimSpace(v) != "" {
		u.APIKey = strings.TrimSpace(v)
	}
	if v, _ := cmd.Flags().GetString("base-url"); strings.TrimSpace(v) != "" {
		if normalized, err := config.NormalizeBaseURL(v); err == nil {
			u.BaseURL = normalized
		}
	}
	return u, nil
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
