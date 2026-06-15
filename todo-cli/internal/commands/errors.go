package commands

import "github.com/graydovee/todolist/todo-cli/internal/client"

type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	return e.Err.Error()
}

func ExitCodeForError(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*ExitError); ok {
		return exitErr.Code
	}
	if apiErr, ok := err.(*client.APIError); ok {
		switch apiErr.Status {
		case 401:
			return 3
		case 403:
			return 4
		case 409:
			return 5
		default:
			return 1
		}
	}
	return 1
}
