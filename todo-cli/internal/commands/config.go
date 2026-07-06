package commands

import (
	"fmt"

	"github.com/graydovee/todo-manager/todo-cli/internal/config"
	"github.com/graydovee/todo-manager/todo-cli/internal/output"
	"github.com/spf13/cobra"
)

func newConfigCommand() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect CLI configuration",
	}
	configCmd.AddCommand(newConfigViewCommand())
	configCmd.AddCommand(newConfigValidateCommand())
	configCmd.AddCommand(newConfigUserCommand())
	return configCmd
}

func newConfigViewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Print the stored CLI configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			if appCtx == nil || appCtx.Config == nil {
				return writeError(cmd, fmt.Errorf("configuration is not loaded"))
			}
			// config view defaults to YAML; only override when -o/--output is set explicitly.
			format := appCtx.Output
			if !cmd.Flags().Changed("output") {
				format = output.FormatYAML
			}
			return output.Write(cmd.OutOrStdout(), format, appCtx.Config.View())
		},
	}
}

func newConfigValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate CLI configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			if appCtx == nil || appCtx.Config == nil {
				return writeError(cmd, fmt.Errorf("configuration is not loaded"))
			}
			effective, err := resolveEffectiveUser(cmd, appCtx.Config)
			if err != nil {
				return writeResult(cmd, map[string]any{"valid": false, "error": err.Error()})
			}
			if verr := config.ValidateUser(effective); verr != nil {
				return writeResult(cmd, map[string]any{
					"valid":    false,
					"user":     effective.Name,
					"error":    verr.Error(),
					"base_url": effective.BaseURL,
					"api_key":  config.MaskAPIKey(effective.APIKey),
				})
			}
			return writeResult(cmd, map[string]any{
				"valid":    true,
				"user":     effective.Name,
				"api_key":  config.MaskAPIKey(effective.APIKey),
				"base_url": effective.BaseURL,
			})
		},
	}
}
