package cursor

import "testing"

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
