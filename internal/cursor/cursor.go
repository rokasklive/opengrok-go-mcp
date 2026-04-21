package cursor

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

type State struct {
	Project    string   `json:"project"`
	Projects   []string `json:"projects,omitempty"`
	Query      string   `json:"query"`
	Mode       string   `json:"mode"`
	Offset     int      `json:"offset"`
	PageSize   int      `json:"page_size"`
	PathPrefix string   `json:"path_prefix,omitempty"`
	FileType   string   `json:"file_type,omitempty"`
}

func Encode(state State) (string, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("marshal cursor state: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}

func Decode(value string) (State, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return State{}, fmt.Errorf("decode cursor base64: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("unmarshal cursor state: %w", err)
	}

	if state.Offset < 0 {
		return State{}, fmt.Errorf("invalid cursor offset %d: must be >= 0", state.Offset)
	}
	if state.PageSize < 1 {
		return State{}, fmt.Errorf("invalid cursor page size %d: must be >= 1", state.PageSize)
	}

	return state, nil
}

func (s State) Validate(expected State) error {
	if s.Project != expected.Project ||
		!slices.Equal(s.Projects, expected.Projects) ||
		s.Query != expected.Query ||
		s.Mode != expected.Mode ||
		s.PathPrefix != expected.PathPrefix ||
		s.FileType != expected.FileType {
		return errors.New("cursor query context does not match expected query context")
	}

	return nil
}
