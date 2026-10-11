// SPDX-License-Identifier: Apache-2.0

package cursor

import (
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	want := State{
		Project:    "platform",
		Projects:   []string{"platform", "tools"},
		Query:      "simulateDepletion",
		Mode:       "full_text",
		Offset:     20,
		PageSize:   20,
		PathPrefix: "src/services/",
		FileType:   "swift",
	}

	encoded, err := Encode(want)
	if err != nil {
		t.Fatalf("Encode() error = %v, want nil", err)
	}

	got, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}

	if got.Project != want.Project ||
		got.Query != want.Query ||
		got.Mode != want.Mode ||
		got.Offset != want.Offset ||
		got.PageSize != want.PageSize ||
		got.PathPrefix != want.PathPrefix ||
		got.FileType != want.FileType ||
		len(got.Projects) != len(want.Projects) ||
		got.Projects[0] != want.Projects[0] ||
		got.Projects[1] != want.Projects[1] {
		t.Fatalf("Decode(Encode(state)) = %#v, want %#v", got, want)
	}
}

func TestValidateRejectsMismatchedQueryContext(t *testing.T) {
	state := State{
		Project:    "platform",
		Projects:   []string{"platform", "tools"},
		Query:      "simulateDepletion",
		Mode:       "full_text",
		Offset:     20,
		PageSize:   20,
		PathPrefix: "src/services/",
		FileType:   "swift",
	}
	expected := State{
		Project:    "platform",
		Projects:   []string{"platform", "tools"},
		Query:      "otherQuery",
		Mode:       "full_text",
		Offset:     0,
		PageSize:   100,
		PathPrefix: "src/services/",
		FileType:   "swift",
	}

	if err := state.Validate(expected); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestValidateRejectsMismatchedProjects(t *testing.T) {
	state := State{
		Project:  "platform",
		Projects: []string{"platform", "tools"},
		Query:    "simulateDepletion",
		Mode:     "full_text",
		Offset:   20,
		PageSize: 20,
	}
	expected := State{
		Project:  "platform",
		Projects: []string{"platform", "other"},
		Query:    "simulateDepletion",
		Mode:     "full_text",
		Offset:   0,
		PageSize: 20,
	}

	if err := state.Validate(expected); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestDecodeRejectsMalformedCursor(t *testing.T) {
	if _, err := Decode("not valid base64!"); err == nil {
		t.Fatal("Decode() error = nil, want error")
	}
}

func TestDecodeRejectsNegativeOffset(t *testing.T) {
	encoded, err := Encode(State{
		Project:  "platform",
		Query:    "simulateDepletion",
		Mode:     "full_text",
		Offset:   -1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("Encode() error = %v, want nil", err)
	}

	if _, err := Decode(encoded); err == nil {
		t.Fatal("Decode() error = nil, want error")
	}
}

func TestDecodeRejectsPageSizeLessThanOne(t *testing.T) {
	encoded, err := Encode(State{
		Project:  "platform",
		Query:    "simulateDepletion",
		Mode:     "full_text",
		Offset:   0,
		PageSize: 0,
	})
	if err != nil {
		t.Fatalf("Encode() error = %v, want nil", err)
	}

	if _, err := Decode(encoded); err == nil {
		t.Fatal("Decode() error = nil, want error")
	}
}

func TestEncodeDecodeProjectsState(t *testing.T) {
	state := ProjectsState{Offset: 50, PageSize: 50}

	encoded, err := EncodeProjects(state)
	if err != nil {
		t.Fatalf("EncodeProjects() error = %v, want nil", err)
	}
	if encoded == "" {
		t.Fatal("EncodeProjects() = empty string, want non-empty")
	}

	decoded, err := DecodeProjects(encoded)
	if err != nil {
		t.Fatalf("DecodeProjects() error = %v, want nil", err)
	}
	if decoded.Offset != state.Offset {
		t.Fatalf("Offset = %d, want %d", decoded.Offset, state.Offset)
	}
	if decoded.PageSize != state.PageSize {
		t.Fatalf("PageSize = %d, want %d", decoded.PageSize, state.PageSize)
	}
}

func TestDecodeProjectsRejectsInvalidBase64(t *testing.T) {
	_, err := DecodeProjects("not-valid-base64!!!")
	if err == nil {
		t.Fatal("DecodeProjects() error = nil, want error")
	}
}

func TestDecodeProjectsRejectsNegativeOffset(t *testing.T) {
	state := ProjectsState{Offset: -1, PageSize: 50}
	encoded, err := EncodeProjects(state)
	if err != nil {
		t.Fatalf("EncodeProjects() error = %v", err)
	}
	_, err = DecodeProjects(encoded)
	if err == nil {
		t.Fatal("DecodeProjects() error = nil, want error for negative offset")
	}
}

func TestDecodeProjectsRejectsZeroPageSize(t *testing.T) {
	state := ProjectsState{Offset: 0, PageSize: 0}
	encoded, err := EncodeProjects(state)
	if err != nil {
		t.Fatalf("EncodeProjects() error = %v", err)
	}
	_, err = DecodeProjects(encoded)
	if err == nil {
		t.Fatal("DecodeProjects() error = nil, want error for zero page size")
	}
}

func TestEncodeDecodeFileState(t *testing.T) {
	want := FileState{
		Project:   "platform",
		FilePath:  "src/Engine.swift",
		StartLine: 500,
		PageSize:  500,
	}

	encoded, err := EncodeFile(want)
	if err != nil {
		t.Fatalf("EncodeFile() error = %v, want nil", err)
	}

	got, err := DecodeFile(encoded)
	if err != nil {
		t.Fatalf("DecodeFile() error = %v, want nil", err)
	}

	if got.Project != want.Project ||
		got.FilePath != want.FilePath ||
		got.StartLine != want.StartLine ||
		got.PageSize != want.PageSize {
		t.Fatalf("DecodeFile(EncodeFile(state)) = %#v, want %#v", got, want)
	}
}

func TestDecodeFileRejectsInvalidBase64(t *testing.T) {
	_, err := DecodeFile("not-valid-base64!!!")
	if err == nil {
		t.Fatal("DecodeFile() error = nil, want error")
	}
}

func TestDecodeFileRejectsStartLineLessThanOne(t *testing.T) {
	for _, startLine := range []int{-1, 0} {
		state := FileState{Project: "platform", FilePath: "src/Engine.swift", StartLine: startLine, PageSize: 500}
		encoded, err := EncodeFile(state)
		if err != nil {
			t.Fatalf("EncodeFile() error = %v", err)
		}
		_, err = DecodeFile(encoded)
		if err == nil {
			t.Fatalf("DecodeFile() error = nil, want error for StartLine = %d", startLine)
		}
	}
}

func TestDecodeFileRejectsZeroPageSize(t *testing.T) {
	state := FileState{Project: "platform", FilePath: "src/Engine.swift", StartLine: 1, PageSize: 0}
	encoded, err := EncodeFile(state)
	if err != nil {
		t.Fatalf("EncodeFile() error = %v", err)
	}
	_, err = DecodeFile(encoded)
	if err == nil {
		t.Fatal("DecodeFile() error = nil, want error for zero page size")
	}
}

func TestEncodeDecodeFileListState(t *testing.T) {
	want := FileListState{
		Project:  "platform",
		Path:     "src/",
		Offset:   25,
		PageSize: 50,
	}

	encoded, err := EncodeFileList(want)
	if err != nil {
		t.Fatalf("EncodeFileList() error = %v, want nil", err)
	}

	got, err := DecodeFileList(encoded)
	if err != nil {
		t.Fatalf("DecodeFileList() error = %v, want nil", err)
	}

	if got.Project != want.Project ||
		got.Path != want.Path ||
		got.Offset != want.Offset ||
		got.PageSize != want.PageSize {
		t.Fatalf("DecodeFileList(EncodeFileList(state)) = %#v, want %#v", got, want)
	}
}

func TestDecodeFileListRejectsNegativeOffset(t *testing.T) {
	state := FileListState{Project: "platform", Path: "src/", Offset: -1, PageSize: 50}
	encoded, err := EncodeFileList(state)
	if err != nil {
		t.Fatalf("EncodeFileList() error = %v", err)
	}
	_, err = DecodeFileList(encoded)
	if err == nil {
		t.Fatal("DecodeFileList() error = nil, want error for negative offset")
	}
}

func TestDecodeFileListRejectsZeroPageSize(t *testing.T) {
	state := FileListState{Project: "platform", Path: "src/", Offset: 0, PageSize: 0}
	encoded, err := EncodeFileList(state)
	if err != nil {
		t.Fatalf("EncodeFileList() error = %v", err)
	}
	_, err = DecodeFileList(encoded)
	if err == nil {
		t.Fatal("DecodeFileList() error = nil, want error for zero page size")
	}
}

func TestDecodeFileListRejectsEmptyProject(t *testing.T) {
	state := FileListState{Project: "", Path: "src/", Offset: 0, PageSize: 50}
	encoded, err := EncodeFileList(state)
	if err != nil {
		t.Fatalf("EncodeFileList() error = %v", err)
	}
	_, err = DecodeFileList(encoded)
	if err == nil {
		t.Fatal("DecodeFileList() error = nil, want error for empty project")
	}
}

func TestDecodeFileListRejectsInvalidBase64(t *testing.T) {
	_, err := DecodeFileList("not-valid-base64!!!")
	if err == nil {
		t.Fatal("DecodeFileList() error = nil, want error")
	}
}

func TestEncodeSignsCursor(t *testing.T) {
	oldSecret := Secret
	Secret = "test-secret"
	defer func() { Secret = oldSecret }()

	state := State{Project: "platform", Query: "test", Mode: "full_text", Offset: 0, PageSize: 20}
	encoded, err := Encode(state)
	if err != nil {
		t.Fatalf("Encode() error = %v, want nil", err)
	}
	if !strings.Contains(encoded, ".") {
		t.Fatal("Encode() = unsigned cursor, want signed cursor with signature")
	}
}

func TestDecodeRejectsTamperedCursor(t *testing.T) {
	oldSecret := Secret
	Secret = "test-secret"
	defer func() { Secret = oldSecret }()

	state := State{Project: "platform", Query: "test", Mode: "full_text", Offset: 0, PageSize: 20}
	encoded, err := Encode(state)
	if err != nil {
		t.Fatalf("Encode() error = %v, want nil", err)
	}

	tampered := encoded[:len(encoded)-4] + "XXXX"
	_, err = Decode(tampered)
	if err == nil {
		t.Fatal("Decode() error = nil, want error for tampered cursor")
	}
	if !strings.Contains(err.Error(), "INVALID_CURSOR: signature mismatch") {
		t.Fatalf("Decode() error = %v, want signature mismatch", err)
	}
}

func TestDecodeAcceptsUnsignedCursorWhenSecretEmpty(t *testing.T) {
	oldSecret := Secret
	Secret = ""
	defer func() { Secret = oldSecret }()

	state := State{Project: "platform", Query: "test", Mode: "full_text", Offset: 0, PageSize: 20}
	encoded, err := Encode(state)
	if err != nil {
		t.Fatalf("Encode() error = %v, want nil", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if decoded.Query != state.Query {
		t.Fatalf("Decode() Query = %q, want %q", decoded.Query, state.Query)
	}
}

func TestDecodeAcceptsSignedCursorWhenSecretMatches(t *testing.T) {
	oldSecret := Secret
	Secret = "matching-secret"
	defer func() { Secret = oldSecret }()

	state := State{Project: "platform", Query: "test", Mode: "full_text", Offset: 0, PageSize: 20}
	encoded, err := Encode(state)
	if err != nil {
		t.Fatalf("Encode() error = %v, want nil", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if decoded.Query != state.Query {
		t.Fatalf("Decode() Query = %q, want %q", decoded.Query, state.Query)
	}
}

func TestDecodeRejectsSignedCursorWhenSecretMismatched(t *testing.T) {
	oldSecret := Secret
	Secret = "correct-secret"
	defer func() { Secret = oldSecret }()

	state := State{Project: "platform", Query: "test", Mode: "full_text", Offset: 0, PageSize: 20}
	encoded, err := Encode(state)
	if err != nil {
		t.Fatalf("Encode() error = %v, want nil", err)
	}

	Secret = "wrong-secret"
	_, err = Decode(encoded)
	if err == nil {
		t.Fatal("Decode() error = nil, want error for mismatched secret")
	}
	if !strings.Contains(err.Error(), "INVALID_CURSOR: signature mismatch") {
		t.Fatalf("Decode() error = %v, want signature mismatch", err)
	}
}

func TestEncodeDecodeKeepsMaxHitsPerFile(t *testing.T) {
	encoded, err := Encode(State{
		Project:        "platform",
		Query:          "simulateDepletion",
		Mode:           "full_text",
		PageSize:       20,
		LineOffset:     3,
		MaxHitsPerFile: 2,
	})
	if err != nil {
		t.Fatalf("Encode() error = %v, want nil", err)
	}

	state, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if state.MaxHitsPerFile != 2 {
		t.Fatalf("MaxHitsPerFile = %d, want 2", state.MaxHitsPerFile)
	}
}

func TestDecodeRejectsNegativeMaxHitsPerFile(t *testing.T) {
	encoded, err := Encode(State{
		Project:        "platform",
		Query:          "simulateDepletion",
		Mode:           "full_text",
		PageSize:       20,
		MaxHitsPerFile: -1,
	})
	if err != nil {
		t.Fatalf("Encode() error = %v, want nil", err)
	}

	if _, err := Decode(encoded); err == nil {
		t.Fatal("Decode() error = nil, want error")
	}
}
