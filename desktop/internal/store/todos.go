package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/graydovee/todo-manager/desktop/internal/client"
)

// TodoStore caches the current list page, the open detail, and its comments.
type TodoStore struct {
	mu sync.Mutex

	// List state.
	Items   []client.Todo
	Total   int64
	Page    int
	Loading bool
	Error   error

	// Detail state (separately fetched for relations).
	Detail        *client.TodoDetail
	DetailLoading bool
	DetailError   error

	// Comments for the currently open detail.
	Comments        []client.Comment
	CommentsLoading bool
}

func NewTodoStore() *TodoStore {
	return &TodoStore{Page: 1}
}

func (t *TodoStore) Lock()   { t.mu.Lock() }
func (t *TodoStore) Unlock() { t.mu.Unlock() }

// Snapshot returns a shallow copy of the current list safe to read on the UI
// goroutine. Pointers inside items are shared; UI must not mutate them.
func (t *TodoStore) Snapshot() (items []client.Todo, total int64, loading bool, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	items = make([]client.Todo, len(t.Items))
	copy(items, t.Items)
	return items, t.Total, t.Loading, t.Error
}

// DetailSnapshot returns the current detail pointer and load state. The pointer
// is shared; callers must not mutate it.
func (t *TodoStore) DetailSnapshot() (detail *client.TodoDetail, loading bool, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Detail, t.DetailLoading, t.DetailError
}

// CommentsSnapshot returns a shallow copy of the cached comments.
func (t *TodoStore) CommentsSnapshot() []client.Comment {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]client.Comment, len(t.Comments))
	copy(out, t.Comments)
	return out
}

// Refresh fetches the list page using query. It runs on a caller-supplied
// context; the onDone callback (may be nil) fires after the result lands.
func (t *TodoStore) Refresh(ctx context.Context, c *client.Client, query map[string][]string, onDone func()) {
	if c == nil {
		return
	}
	t.mu.Lock()
	t.Loading = true
	t.Error = nil
	t.mu.Unlock()

	go func() {
		resp, err := c.ListTodos(ctx, query)
		t.mu.Lock()
		t.Loading = false
		if err != nil {
			t.Error = err
			t.Items = nil
		} else {
			t.Error = nil
			t.Items = resp.Items
			t.Total = resp.Total
			t.Page = resp.Page
		}
		t.mu.Unlock()
		if onDone != nil {
			onDone()
		}
	}()
}

// LoadDetail fetches a single todo's full detail and its comments together.
func (t *TodoStore) LoadDetail(ctx context.Context, c *client.Client, id string, onDone func()) {
	if c == nil {
		return
	}
	t.mu.Lock()
	t.DetailLoading = true
	t.CommentsLoading = true
	t.DetailError = nil
	t.mu.Unlock()

	go func() {
		detail, err := c.GetTodo(ctx, id)
		t.mu.Lock()
		t.DetailLoading = false
		if err != nil {
			t.DetailError = err
			t.Detail = nil
		} else {
			t.Detail = detail
		}
		t.mu.Unlock()

		// Fetch comments best-effort.
		comments, cerr := c.ListComments(ctx, id)
		t.mu.Lock()
		t.CommentsLoading = false
		if cerr == nil {
			t.Comments = comments
		} else {
			t.Comments = nil
		}
		t.mu.Unlock()

		if onDone != nil {
			onDone()
		}
	}()
}

// ResetDetail clears the detail + comments cache.
func (t *TodoStore) ResetDetail() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Detail = nil
	t.DetailError = nil
	t.Comments = nil
}

// IDString returns the selected id formatted for API calls (helper for UI).
func IDString(id uint) string {
	if id == 0 {
		return ""
	}
	return fmt.Sprintf("%d", id)
}
