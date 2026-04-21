package links

import "testing"

func TestBuilderFileBuildsDisplayRawAndResourceLinks(t *testing.T) {
	fileLinks := NewBuilder("https://grok.example.com/source", true).File(
		"platform",
		"src/services/Engine.swift",
		42,
	)

	wantDisplayURL := "https://grok.example.com/source/xref/platform/src/services/Engine.swift#42"
	if fileLinks.DisplayURL != wantDisplayURL {
		t.Fatalf("DisplayURL = %q, want %q", fileLinks.DisplayURL, wantDisplayURL)
	}
	if fileLinks.RawURL == nil {
		t.Fatal("RawURL = nil, want URL")
	}

	wantRawURL := "https://grok.example.com/source/raw/platform/src/services/Engine.swift"
	if *fileLinks.RawURL != wantRawURL {
		t.Fatalf("RawURL = %q, want %q", *fileLinks.RawURL, wantRawURL)
	}

	wantResourceURI := "opengrok://project/platform/files/src/services/Engine.swift#L42"
	if fileLinks.ResourceURI != wantResourceURI {
		t.Fatalf("ResourceURI = %q, want %q", fileLinks.ResourceURI, wantResourceURI)
	}
}

func TestBuilderSearchBuildsFullTextURLWithTrimmedBaseURL(t *testing.T) {
	got := NewBuilder("https://grok.example.com/source/", true).Search(
		"platform",
		"full_text",
		"simulate depletion",
	)
	want := "https://grok.example.com/source/search?full=simulate+depletion&project=platform"

	if got != want {
		t.Fatalf("Search() = %q, want %q", got, want)
	}
}

func TestBuilderProjectBuildsResourceURI(t *testing.T) {
	got := NewBuilder("https://grok.example.com/source", true).Project("mobile platform")
	want := "opengrok://project/mobile%20platform"

	if got != want {
		t.Fatalf("Project() = %q, want %q", got, want)
	}
}

func TestBuilderEscapesPathSegmentsAndPreservesSlashes(t *testing.T) {
	fileLinks := NewBuilder("https://grok.example.com/source", true).File(
		"mobile platform",
		"src/My Services/Engine+Runner.swift",
		0,
	)

	wantDisplayURL := "https://grok.example.com/source/xref/mobile%20platform/src/My%20Services/Engine+Runner.swift"
	if fileLinks.DisplayURL != wantDisplayURL {
		t.Fatalf("DisplayURL = %q, want %q", fileLinks.DisplayURL, wantDisplayURL)
	}

	wantResourceURI := "opengrok://project/mobile%20platform/files/src/My%20Services/Engine+Runner.swift"
	if fileLinks.ResourceURI != wantResourceURI {
		t.Fatalf("ResourceURI = %q, want %q", fileLinks.ResourceURI, wantResourceURI)
	}
}

func TestBuilderFileOmitsRawURLWhenDisabled(t *testing.T) {
	fileLinks := NewBuilder("https://grok.example.com/source", false).File(
		"platform",
		"src/services/Engine.swift",
		42,
	)

	if fileLinks.RawURL != nil {
		t.Fatalf("RawURL = %q, want nil", *fileLinks.RawURL)
	}
}
