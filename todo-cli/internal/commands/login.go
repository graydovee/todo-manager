package commands

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/graydovee/todolist/todo-cli/internal/config"
	"github.com/spf13/cobra"
)

func newLoginCommand() *cobra.Command {
	var (
		apiKeyFlag  string
		baseURLFlag string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Generate and save CLI configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			apiKey := strings.TrimSpace(apiKeyFlag)
			if apiKey == "" {
				reader := bufio.NewReader(cmd.InOrStdin())
				_, _ = fmt.Fprint(cmd.ErrOrStderr(), "Enter API key: ")
				line, err := reader.ReadString('\n')
				if err != nil && strings.TrimSpace(line) == "" {
					return writeError(cmd, &ExitError{Code: 2, Err: fmt.Errorf("api_key is required")})
				}
				apiKey = strings.TrimSpace(line)
			}

			baseURL := strings.TrimSpace(baseURLFlag)
			if baseURL == "" {
				baseURL = config.DefaultBaseURL
			}
			normalizedBaseURL, err := config.NormalizeBaseURL(baseURL)
			if err != nil {
				return writeError(cmd, &ExitError{Code: 2, Err: err})
			}

			cfg := &config.Config{
				APIKey:  apiKey,
				BaseURL: normalizedBaseURL,
			}
			if err := config.Validate(cfg); err != nil {
				return writeError(cmd, &ExitError{Code: 2, Err: err})
			}
			if err := config.Write("", cfg); err != nil {
				return writeError(cmd, err)
			}
			data, err := config.MarshalYAML(cfg)
			if err != nil {
				return writeError(cmd, err)
			}
			if _, err := cmd.OutOrStdout().Write(data); err != nil {
				return writeError(cmd, err)
			}
			homeDir, err := configUserHomeDir()
			if err == nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Wrote configuration to %s\n", config.ConfigPath(homeDir))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&apiKeyFlag, "api-key", "", "Access key to save into config")
	cmd.Flags().StringVar(&baseURLFlag, "base-url", "", "Todo Manager server base URL")
	return cmd
}
