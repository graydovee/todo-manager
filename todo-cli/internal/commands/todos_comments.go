package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newTodosCommentsCommand() *cobra.Command {
	commentsCmd := &cobra.Command{
		Use:   "comments",
		Short: "Manage todo comments",
	}
	commentsCmd.AddCommand(&cobra.Command{
		Use:   "list <todo-id>",
		Short: "List comments for a todo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			result, err := appCtx.Client.ListComments(commandContext(cmd), args[0])
			if err != nil {
				return writeError(cmd, err)
			}
			return writeResult(cmd, result)
		},
	})
	createCmd := &cobra.Command{
		Use:   "create <todo-id>",
		Short: "Create a comment for a todo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, _ := cmd.Flags().GetString("content")
			if strings.TrimSpace(content) == "" {
				return writeError(cmd, &ExitError{Code: 2, Err: fmt.Errorf("--content is required")})
			}
			appCtx := getAppContext(cmd)
			result, err := appCtx.Client.CreateComment(commandContext(cmd), args[0], map[string]any{"content": content})
			if err != nil {
				return writeError(cmd, err)
			}
			return writeResult(cmd, result)
		},
	}
	createCmd.Flags().String("content", "", "Comment content")
	commentsCmd.AddCommand(createCmd)
	commentsCmd.AddCommand(&cobra.Command{
		Use:   "delete <todo-id> <comment-id>",
		Short: "Delete a comment from a todo",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			result, err := appCtx.Client.DeleteComment(commandContext(cmd), args[0], args[1])
			if err != nil {
				return writeError(cmd, err)
			}
			return writeResult(cmd, result)
		},
	})
	return commentsCmd
}
