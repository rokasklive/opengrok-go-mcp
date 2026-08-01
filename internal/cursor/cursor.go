// SPDX-License-Identifier: Apache-2.0

package cursor

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var Secret string

func signPayload(data []byte) string {
	payload := base64.RawURLEncoding.EncodeToString(data)
	if Secret == "" {
		return payload
	}
	mac := hmac.New(sha256.New, []byte(Secret))
	mac.Write(data)
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

func verifyPayload(value string) ([]byte, error) {
	dotIdx := strings.LastIndex(value, ".")
	if Secret != "" && dotIdx < 0 {
		return nil, errors.New("INVALID_CURSOR: signature required when secret is configured")
	}

	payload := value
	if dotIdx >= 0 {
		payload = value[:dotIdx]
	}

	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("decode cursor base64: %w", err)
	}

	if Secret != "" {
		mac := hmac.New(sha256.New, []byte(Secret))
		mac.Write(data)
		expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(value[dotIdx+1:]), []byte(expected)) {
			return nil, errors.New("INVALID_CURSOR: signature mismatch")
		}
	}

	return data, nil
}

type State struct {
	Project    string   `json:"project"`
	Projects   []string `json:"projects,omitempty"`
	Query      string   `json:"query"`
	Mode       string   `json:"mode"`
	Offset     int      `json:"offset"`
	PageSize   int      `json:"page_size"`
	// LineOffset resumes within the flattened line hits of the file window that
	// starts at Offset. OpenGrok pages by file, but a page is capped at
	// PageSize *lines*, so a file window can hold more lines than one page can
	// return. Without this the next cursor skipped the whole window and those
	// lines were unreachable. Absent in cursors minted before this field
	// existed, which decode to 0 — the old first-page-of-window behavior.
	LineOffset int `json:"line_offset,omitempty"`
	PathPrefix string   `json:"path_prefix,omitempty"`
	FileType   string   `json:"file_type,omitempty"`
}

func Encode(state State) (string, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("marshal cursor state: %w", err)
	}
	return signPayload(data), nil
}

func Decode(value string) (State, error) {
	data, err := verifyPayload(value)
	if err != nil {
		return State{}, err
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
	if state.LineOffset < 0 {
		return State{}, fmt.Errorf("invalid cursor line offset %d: must be >= 0", state.LineOffset)
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

type ProjectsState struct {
	Offset   int `json:"offset"`
	PageSize int `json:"page_size"`
}

func EncodeProjects(state ProjectsState) (string, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("marshal projects cursor state: %w", err)
	}
	return signPayload(data), nil
}

func DecodeProjects(value string) (ProjectsState, error) {
	data, err := verifyPayload(value)
	if err != nil {
		return ProjectsState{}, err
	}
	var state ProjectsState
	if err := json.Unmarshal(data, &state); err != nil {
		return ProjectsState{}, fmt.Errorf("unmarshal projects cursor state: %w", err)
	}
	if state.Offset < 0 {
		return ProjectsState{}, fmt.Errorf("invalid cursor offset %d: must be >= 0", state.Offset)
	}
	if state.PageSize < 1 {
		return ProjectsState{}, fmt.Errorf("invalid cursor page size %d: must be >= 1", state.PageSize)
	}
	return state, nil
}

type FileState struct {
	Project   string `json:"project"`
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line"`
	PageSize  int    `json:"page_size"`
}

func EncodeFile(state FileState) (string, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("marshal file cursor state: %w", err)
	}
	return signPayload(data), nil
}

func DecodeFile(value string) (FileState, error) {
	data, err := verifyPayload(value)
	if err != nil {
		return FileState{}, err
	}
	var state FileState
	if err := json.Unmarshal(data, &state); err != nil {
		return FileState{}, fmt.Errorf("unmarshal file cursor state: %w", err)
	}
	if state.StartLine < 1 {
		return FileState{}, fmt.Errorf("invalid cursor start line %d: must be >= 1", state.StartLine)
	}
	if state.PageSize < 1 {
		return FileState{}, fmt.Errorf("invalid cursor page size %d: must be >= 1", state.PageSize)
	}
	return state, nil
}

type FileListState struct {
	Project  string `json:"project"`
	Path     string `json:"path"`
	Offset   int    `json:"offset"`
	PageSize int    `json:"page_size"`
}

func EncodeFileList(state FileListState) (string, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("marshal file list cursor state: %w", err)
	}
	return signPayload(data), nil
}

func DecodeFileList(value string) (FileListState, error) {
	data, err := verifyPayload(value)
	if err != nil {
		return FileListState{}, err
	}
	var state FileListState
	if err := json.Unmarshal(data, &state); err != nil {
		return FileListState{}, fmt.Errorf("unmarshal file list cursor state: %w", err)
	}
	if state.Project == "" {
		return FileListState{}, fmt.Errorf("invalid cursor project: must not be empty")
	}
	if state.Offset < 0 {
		return FileListState{}, fmt.Errorf("invalid cursor offset %d: must be >= 0", state.Offset)
	}
	if state.PageSize < 1 {
		return FileListState{}, fmt.Errorf("invalid cursor page size %d: must be >= 1", state.PageSize)
	}
	return state, nil
}
