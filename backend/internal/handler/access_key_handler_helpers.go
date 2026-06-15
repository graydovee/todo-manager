package handler

import (
	"encoding/json"
	"time"

	"github.com/graydovee/todolist/internal/model"
)

func decodeAuthorizedAPIs(key *model.AccessKey) ([]string, error) {
	var authorized []string
	if err := json.Unmarshal([]byte(key.AuthorizedAPIsJSON), &authorized); err != nil {
		return nil, err
	}
	return authorized, nil
}

func parseOptionalTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
