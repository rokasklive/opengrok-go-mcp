// SPDX-License-Identifier: Apache-2.0

package opengrok

import (
	"net/http"
	"strings"
	"testing"
)

// OpenGrok markup may omit the optional </option> close tag. The streaming
// html.Tokenizer does no implicit tag closing, so options must be flushed on the
// next <option> start and on </select>, not only on </option>.
func TestParseProjectSelectOptionsImplicitOptionClose(t *testing.T) {
	const withValues = `<select id="project" name="project" multiple>
<option value="alpha">alpha
<option value="beta">beta
</select>`
	if got := parseProjectSelectOptions(strings.NewReader(withValues)); !slicesEqualStrings(got, []string{"alpha", "beta"}) {
		t.Fatalf("implicit close (value attrs) = %#v, want [alpha beta]", got)
	}

	const textOnly = `<select id="project"><option>gamma<option>delta</select>`
	if got := parseProjectSelectOptions(strings.NewReader(textOnly)); !slicesEqualStrings(got, []string{"gamma", "delta"}) {
		t.Fatalf("implicit close (text only) = %#v, want [gamma delta]", got)
	}
}

func TestLimitErrorReaderAllowsExactlyMaxBytes(t *testing.T) {
	body := strings.Repeat("x", 1024)
	limited := &limitErrorReader{r: strings.NewReader(body), max: 1024}
	buf := make([]byte, 256)
	for {
		_, err := limited.Read(buf)
		if err != nil {
			break
		}
	}
	if limited.Err() != nil {
		t.Fatalf("Err() = %v, want nil for a body of exactly max bytes", limited.Err())
	}
}

func TestSetDefaultProjectUpdatesAttributionDefault(t *testing.T) {
	c := NewClient("https://grok.example.com/api/v1", http.DefaultClient)
	c.SetDefaultProject("discovered")
	if c.defaultProject != "discovered" {
		t.Fatalf("defaultProject = %q, want discovered", c.defaultProject)
	}
}
