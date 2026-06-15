package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newTodosCommand() *cobra.Command {
	todosCmd := &cobra.Command{
		Use:   "todos",
		Short: "Manage todos",
	}
	todosCmd.AddCommand(newTodosListCommand())
	todosCmd.AddCommand(newTodosGetCommand())
	todosCmd.AddCommand(newTodosCreateCommand())
	todosCmd.AddCommand(newTodosUpdateCommand())
	todosCmd.AddCommand(newTodosDeleteCommand())
	todosCmd.AddCommand(newTodosStartCommand())
	todosCmd.AddCommand(newTodosStatusCommand())
	todosCmd.AddCommand(newTodosCompleteCommand())
	todosCmd.AddCommand(newTodosReopenCommand())
	todosCmd.AddCommand(newTodosPinCommand())
	todosCmd.AddCommand(newTodosHighlightCommand())
	todosCmd.AddCommand(newTodosByDateRangeCommand())
	todosCmd.AddCommand(newTodosTagsCommand())
	todosCmd.AddCommand(newTodosGraphCommand())
	todosCmd.AddCommand(newTodosCommentsCommand())
	return todosCmd
}

func newTodosListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List todos",
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			query := map[string][]string{}
			addStringFlagQuery(cmd, query, "q")
			addStringFlagQuery(cmd, query, "code")
			addStringFlagQuery(cmd, query, "category")
			addStringFlagQuery(cmd, query, "priority")
			addStringFlagQuery(cmd, query, "status")
			addStringFlagQuery(cmd, query, "updated-after")
			addStringFlagQuery(cmd, query, "sort-by")
			addStringFlagQuery(cmd, query, "sort-order")
			if tags := mustStringSliceFlag(cmd, "tag"); len(tags) > 0 {
				query["tag"] = []string{strings.Join(tags, ",")}
			}
			addIntFlagQuery(cmd, query, "page")
			addIntFlagQuery(cmd, query, "page-size")

			result, err := appCtx.Client.ListTodos(commandContext(cmd), query)
			if err != nil {
				return writeError(cmd, err)
			}
			return writeResult(cmd, result)
		},
	}
	cmd.Flags().String("q", "", "Search query")
	cmd.Flags().String("code", "", "Exact todo code")
	cmd.Flags().String("category", "", "Category filter")
	cmd.Flags().String("priority", "", "Priority filter")
	cmd.Flags().String("status", "", "Status filter")
	cmd.Flags().StringSlice("tag", nil, "Tag filter, repeatable")
	cmd.Flags().String("updated-after", "", "Updated after (RFC3339)")
	cmd.Flags().Int("page", 0, "Page number")
	cmd.Flags().Int("page-size", 0, "Page size")
	cmd.Flags().String("sort-by", "", "Sort field")
	cmd.Flags().String("sort-order", "", "Sort order")
	return cmd
}

func newTodosGetCommand() *cobra.Command {
	return singleIDCommand("get <id>", "Get a todo", func(cmd *cobra.Command, id string) error {
		appCtx := getAppContext(cmd)
		result, err := appCtx.Client.GetTodo(commandContext(cmd), id)
		if err != nil {
			return writeError(cmd, err)
		}
		return writeResult(cmd, result)
	})
}

func newTodosCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a todo",
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			body, err := buildTodoBodyFromFlags(cmd, true)
			if err != nil {
				return writeError(cmd, &ExitError{Code: 2, Err: err})
			}
			result, err := appCtx.Client.CreateTodo(commandContext(cmd), body)
			if err != nil {
				return writeError(cmd, err)
			}
			return writeResult(cmd, result)
		},
	}
	addTodoMutationFlags(cmd, true)
	return cmd
}

func newTodosUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a todo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			body, err := buildTodoBodyFromFlags(cmd, false)
			if err != nil {
				return writeError(cmd, &ExitError{Code: 2, Err: err})
			}
			result, err := appCtx.Client.UpdateTodo(commandContext(cmd), args[0], body)
			if err != nil {
				return writeError(cmd, err)
			}
			return writeResult(cmd, result)
		},
	}
	addTodoMutationFlags(cmd, false)
	return cmd
}

func newTodosDeleteCommand() *cobra.Command {
	return singleIDCommand("delete <id>", "Delete a todo", func(cmd *cobra.Command, id string) error {
		appCtx := getAppContext(cmd)
		result, err := appCtx.Client.DeleteTodo(commandContext(cmd), id)
		if err != nil {
			return writeError(cmd, err)
		}
		return writeResult(cmd, result)
	})
}

func newTodosStartCommand() *cobra.Command {
	return singleIDCommand("start <id>", "Start a todo", func(cmd *cobra.Command, id string) error {
		appCtx := getAppContext(cmd)
		result, err := appCtx.Client.StartTodo(commandContext(cmd), id)
		if err != nil {
			return writeError(cmd, err)
		}
		return writeResult(cmd, result)
	})
}

func newTodosStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <id>",
		Short: "Set todo status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			status, _ := cmd.Flags().GetString("status")
			if strings.TrimSpace(status) == "" {
				return writeError(cmd, &ExitError{Code: 2, Err: fmt.Errorf("--status is required")})
			}
			appCtx := getAppContext(cmd)
			result, err := appCtx.Client.SetTodoStatus(commandContext(cmd), args[0], map[string]any{"status": status})
			if err != nil {
				return writeError(cmd, err)
			}
			return writeResult(cmd, result)
		},
	}
	cmd.Flags().String("status", "", "New status")
	return cmd
}

func newTodosCompleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "complete <id>",
		Short: "Complete a todo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			cascade, _ := cmd.Flags().GetBool("cascade-dependencies")
			result, err := appCtx.Client.CompleteTodo(commandContext(cmd), args[0], map[string]any{"cascade_dependencies": cascade})
			if err != nil {
				return writeError(cmd, err)
			}
			return writeResult(cmd, result)
		},
	}
	cmd.Flags().Bool("cascade-dependencies", false, "Cascade to dependencies")
	return cmd
}

func newTodosReopenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reopen <id>",
		Short: "Reopen a todo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			cascade, _ := cmd.Flags().GetBool("cascade-dependents")
			result, err := appCtx.Client.ReopenTodo(commandContext(cmd), args[0], map[string]any{"cascade_dependents": cascade})
			if err != nil {
				return writeError(cmd, err)
			}
			return writeResult(cmd, result)
		},
	}
	cmd.Flags().Bool("cascade-dependents", false, "Cascade to dependents")
	return cmd
}

func newTodosPinCommand() *cobra.Command {
	return booleanMutationCommand("pin <id>", "Pin or unpin a todo", "value", func(cmd *cobra.Command, id string, value bool) error {
		appCtx := getAppContext(cmd)
		result, err := appCtx.Client.PinTodo(commandContext(cmd), id, map[string]any{"pinned": value})
		if err != nil {
			return writeError(cmd, err)
		}
		return writeResult(cmd, result)
	})
}

func newTodosHighlightCommand() *cobra.Command {
	return booleanMutationCommand("highlight <id>", "Highlight or unhighlight a todo", "value", func(cmd *cobra.Command, id string, value bool) error {
		appCtx := getAppContext(cmd)
		result, err := appCtx.Client.HighlightTodo(commandContext(cmd), id, map[string]any{"highlighted": value})
		if err != nil {
			return writeError(cmd, err)
		}
		return writeResult(cmd, result)
	})
}

func newTodosTagsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "tags",
		Short: "List distinct tags",
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			result, err := appCtx.Client.ListTags(commandContext(cmd))
			if err != nil {
				return writeError(cmd, err)
			}
			return writeResult(cmd, result)
		},
	}
}

func newTodosGraphCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "graph",
		Short: "Get todo graph",
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			result, err := appCtx.Client.GetGraph(commandContext(cmd))
			if err != nil {
				return writeError(cmd, err)
			}
			return writeResult(cmd, result)
		},
	}
}

func newTodosByDateRangeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "by-date-range",
		Short: "List todos by updated date range",
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			startDate, _ := cmd.Flags().GetString("start-date")
			endDate, _ := cmd.Flags().GetString("end-date")
			if strings.TrimSpace(startDate) == "" || strings.TrimSpace(endDate) == "" {
				return writeError(cmd, &ExitError{Code: 2, Err: fmt.Errorf("--start-date and --end-date are required")})
			}
			result, err := appCtx.Client.ListByDateRange(commandContext(cmd), map[string][]string{
				"start_date": {startDate},
				"end_date":   {endDate},
			})
			if err != nil {
				return writeError(cmd, err)
			}
			return writeResult(cmd, result)
		},
	}
	cmd.Flags().String("start-date", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().String("end-date", "", "End date (YYYY-MM-DD)")
	return cmd
}

func addStringFlagQuery(cmd *cobra.Command, query map[string][]string, name string) {
	value, _ := cmd.Flags().GetString(name)
	if strings.TrimSpace(value) != "" {
		query[strings.ReplaceAll(name, "-", "_")] = []string{value}
	}
}

func addIntFlagQuery(cmd *cobra.Command, query map[string][]string, name string) {
	value, _ := cmd.Flags().GetInt(name)
	if value > 0 {
		query[strings.ReplaceAll(name, "-", "_")] = []string{strconv.Itoa(value)}
	}
}

func addTodoMutationFlags(cmd *cobra.Command, required bool) {
	cmd.Flags().String("title", "", "Todo title")
	cmd.Flags().String("description", "", "Todo description")
	cmd.Flags().String("category", "", "Todo category")
	cmd.Flags().String("priority", "", "Todo priority")
	cmd.Flags().StringSlice("tag", nil, "Tags, repeatable")
	cmd.Flags().String("due-at", "", "Due at (RFC3339)")
	cmd.Flags().StringSlice("depends-on", nil, "Depends on todo IDs, repeatable")
	cmd.Flags().Uint("duplicate-of", 0, "Duplicate-of todo ID")
	if required {
		_ = cmd.MarkFlagRequired("title")
		_ = cmd.MarkFlagRequired("category")
	}
}

func buildTodoBodyFromFlags(cmd *cobra.Command, creating bool) (map[string]any, error) {
	body := map[string]any{}
	title, _ := cmd.Flags().GetString("title")
	description, _ := cmd.Flags().GetString("description")
	category, _ := cmd.Flags().GetString("category")
	priority, _ := cmd.Flags().GetString("priority")
	tags, _ := cmd.Flags().GetStringSlice("tag")
	dueAt, _ := cmd.Flags().GetString("due-at")
	dependsOn, _ := cmd.Flags().GetStringSlice("depends-on")
	duplicateOf, _ := cmd.Flags().GetUint("duplicate-of")

	if creating || cmd.Flags().Changed("title") {
		body["title"] = title
	}
	if creating || cmd.Flags().Changed("description") {
		body["description"] = description
	}
	if creating || cmd.Flags().Changed("category") {
		body["category"] = category
	}
	if creating || cmd.Flags().Changed("priority") {
		body["priority"] = priority
	}
	if creating || cmd.Flags().Changed("tag") {
		body["tags"] = tags
	}
	if creating || cmd.Flags().Changed("due-at") {
		if strings.TrimSpace(dueAt) == "" {
			body["due_at"] = nil
		} else {
			body["due_at"] = dueAt
		}
	}
	if creating || cmd.Flags().Changed("depends-on") {
		ids, err := parseUintStrings(dependsOn)
		if err != nil {
			return nil, fmt.Errorf("parse --depends-on: %w", err)
		}
		body["depends_on_ids"] = ids
	}
	if creating || cmd.Flags().Changed("duplicate-of") {
		if duplicateOf == 0 {
			body["duplicate_of_id"] = nil
		} else {
			body["duplicate_of_id"] = duplicateOf
		}
	}
	return body, nil
}

func parseUintStrings(values []string) ([]uint, error) {
	result := make([]uint, 0, len(values))
	for _, value := range values {
		id, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, err
		}
		result = append(result, uint(id))
	}
	return result, nil
}

func singleIDCommand(use, short string, run func(cmd *cobra.Command, id string) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, args[0])
		},
	}
}

func booleanMutationCommand(use, short, flagName string, run func(cmd *cobra.Command, id string, value bool) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			value, _ := cmd.Flags().GetBool(flagName)
			return run(cmd, args[0], value)
		},
	}
	cmd.Flags().Bool(flagName, false, "Boolean value")
	return cmd
}
