package main

import (
	"os"

	"github.com/graydovee/todo-manager/todo-cli/internal/commands"
)

func main() {
	if err := commands.NewRootCommand().Execute(); err != nil {
		os.Exit(commands.ExitCodeForError(err))
	}
}
