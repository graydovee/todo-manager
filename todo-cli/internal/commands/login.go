package commands

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/graydovee/todo-manager/todo-cli/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newLoginCommand() *cobra.Command {
	var (
		apiKeyFlag  string
		baseURLFlag string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Create or update a user profile and save configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			if appCtx == nil || appCtx.Config == nil {
				return writeError(cmd, fmt.Errorf("configuration is not loaded"))
			}
			cfg := appCtx.Config

			userFlag, _ := cmd.Flags().GetString("user")
			name := strings.TrimSpace(userFlag)
			bootstrap := name == ""
			if bootstrap {
				if cfg.Auth.DefaultUser != "" {
					return writeError(cmd, &ExitError{Code: 2,
						Err: errors.New("default user already set; use -u <name> to add another")})
				}
				name = "default"
			}

			apiKey := strings.TrimSpace(apiKeyFlag)
			if apiKey == "" {
				reader := bufio.NewReader(cmd.InOrStdin())
				_, _ = fmt.Fprint(cmd.ErrOrStderr(), "Enter API key: ")
				line, _ := reader.ReadString('\n')
				apiKey = strings.TrimSpace(line)
				if apiKey == "" {
					return writeError(cmd, &ExitError{Code: 2, Err: fmt.Errorf("api_key is required")})
				}
			}

			baseURL := strings.TrimSpace(baseURLFlag)
			if baseURL == "" {
				baseURL = config.DefaultBaseURL
			}
			normalizedBaseURL, err := config.NormalizeBaseURL(baseURL)
			if err != nil {
				return writeError(cmd, &ExitError{Code: 2, Err: err})
			}

			candidate := config.UserEntry{
				Name:    name,
				APIKey:  apiKey,
				BaseURL: normalizedBaseURL,
			}
			if err := config.ValidateUser(candidate); err != nil {
				return writeError(cmd, &ExitError{Code: 2, Err: err})
			}

			// UpsertUser overwrites an existing entry with the same name in place.
			cfg.UpsertUser(candidate)
			if bootstrap {
				cfg.Auth.DefaultUser = "default"
			}

			if err := config.Write("", cfg); err != nil {
				return writeError(cmd, err)
			}

			// Print the target user entry (clear api_key) — mirrors the old
			// single-user behavior of echoing the just-entered key.
			data, err := yaml.Marshal(candidate)
			if err != nil {
				return writeError(cmd, err)
			}
			if _, err := cmd.OutOrStdout().Write(append(data, '\n')); err != nil {
				return writeError(cmd, err)
			}
			if homeDir, err := configUserHomeDir(); err == nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Wrote configuration for user %q to %s\n", name, config.ConfigPath(homeDir))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&apiKeyFlag, "api-key", "", "Access key to save for this user")
	cmd.Flags().StringVar(&baseURLFlag, "base-url", "", "Todo Manager server base URL")
	return cmd
}
