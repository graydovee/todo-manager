package commands

import (
	"fmt"
	"strings"

	"github.com/graydovee/todo-manager/todo-cli/internal/config"
	"github.com/spf13/cobra"
)

func newConfigUserCommand() *cobra.Command {
	userCmd := &cobra.Command{
		Use:   "user",
		Short: "Manage user profiles",
	}
	userCmd.AddCommand(newConfigUserListCommand())
	userCmd.AddCommand(newConfigUserSetDefaultCommand())
	userCmd.AddCommand(newConfigUserRemoveCommand())
	userCmd.AddCommand(newConfigUserRenameCommand())
	return userCmd
}

func newConfigUserListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured user profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			if appCtx == nil || appCtx.Config == nil {
				return writeError(cmd, fmt.Errorf("configuration is not loaded"))
			}
			return writeResult(cmd, appCtx.Config.UserList())
		},
	}
}

func newConfigUserSetDefaultCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-default [name]",
		Short: "Set the default user profile (pass \"\" to clear)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			if appCtx == nil || appCtx.Config == nil {
				return writeError(cmd, fmt.Errorf("configuration is not loaded"))
			}
			name := strings.TrimSpace(args[0])
			if err := appCtx.Config.SetDefault(name); err != nil {
				return writeError(cmd, &ExitError{Code: 2, Err: err})
			}
			if err := config.Write("", appCtx.Config); err != nil {
				return writeError(cmd, err)
			}
			return writeResult(cmd, map[string]any{"default_user": name})
		},
	}
}

func newConfigUserRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [name]",
		Short: "Remove a user profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			if appCtx == nil || appCtx.Config == nil {
				return writeError(cmd, fmt.Errorf("configuration is not loaded"))
			}
			name := strings.TrimSpace(args[0])
			if !appCtx.Config.RemoveUser(name) {
				return writeError(cmd, &ExitError{Code: 2, Err: fmt.Errorf("user %q not found", name)})
			}
			if err := config.Write("", appCtx.Config); err != nil {
				return writeError(cmd, err)
			}
			result := map[string]any{"removed": name}
			if appCtx.Config.Auth.DefaultUser == "" {
				result["default_cleared"] = true
			}
			return writeResult(cmd, result)
		},
	}
}

func newConfigUserRenameCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rename [old] [new]",
		Short: "Rename a user profile",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			if appCtx == nil || appCtx.Config == nil {
				return writeError(cmd, fmt.Errorf("configuration is not loaded"))
			}
			oldName := strings.TrimSpace(args[0])
			newName := strings.TrimSpace(args[1])
			if err := appCtx.Config.RenameUser(oldName, newName); err != nil {
				return writeError(cmd, &ExitError{Code: 2, Err: err})
			}
			if err := config.Write("", appCtx.Config); err != nil {
				return writeError(cmd, err)
			}
			return writeResult(cmd, map[string]any{
				"renamed_from": oldName,
				"renamed_to":   newName,
			})
		},
	}
}
