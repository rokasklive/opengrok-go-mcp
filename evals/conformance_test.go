// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/mcpserver"
)

type conformanceCase struct {
	ClaimID        string
	Args           map[string]any
	ControlArgs    map[string]any
	WantErrorCode  string
	ControlMayFail bool
	Conditional    bool
}

func TestLiveConformanceClaimCoverage(t *testing.T) {
	cases := conformanceCases()
	caseIDs := map[string]bool{}
	for _, tc := range cases {
		if caseIDs[tc.ClaimID] {
			t.Fatalf("duplicate conformance case for claim %q", tc.ClaimID)
		}
		caseIDs[tc.ClaimID] = true
	}

	for _, claim := range mcpserver.ClaimsByGate(mcpserver.ClaimGateLive) {
		if !caseIDs[claim.ID] {
			t.Errorf("live claim %q has no conformance case", claim.ID)
		}
	}
	for claimID := range caseIDs {
		claim, ok := mcpserver.ClaimByID(claimID)
		if !ok {
			t.Errorf("conformance case references unknown claim %q", claimID)
			continue
		}
		if claim.Gate != mcpserver.ClaimGateLive {
			t.Errorf("conformance case %q has gate %q, want live", claimID, claim.Gate)
		}
	}
}

func TestLiveConformance(t *testing.T) {
	if os.Getenv("OPENGROK_MCP_LIVE_EVAL") != "1" {
		t.Skip("set OPENGROK_MCP_LIVE_EVAL=1 to run against a live OpenGrok instance")
	}
	if os.Getenv("OPENGROK_MCP_BASE_URL") == "" {
		t.Skip("OPENGROK_MCP_BASE_URL required for live eval")
	}

	ctx := context.Background()
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	h, err := StartLive(ctx, moduleRoot, HarnessOptions{ToolSurface: surfaceCompact, AgentProfile: "economy"})
	if err != nil {
		t.Fatalf("start live harness: %v", err)
	}
	defer h.Stop()

	for _, tc := range conformanceCases() {
		t.Run(tc.ClaimID, func(t *testing.T) {
			claim, ok := mcpserver.ClaimByID(tc.ClaimID)
			if !ok {
				t.Fatalf("unknown claim %q", tc.ClaimID)
			}
			out := callConformanceTool(t, ctx, h, tc.Args)
			if tc.WantErrorCode != "" {
				assertConformanceError(t, tc.ClaimID, out, tc.WantErrorCode)
			} else if out.IsError {
				if tc.Conditional {
					t.Skipf("conditional claim %s not accepted by this OpenGrok deployment: %s", tc.ClaimID, contentText(out))
				}
				t.Fatalf("claim %s positive example returned an error: %s", tc.ClaimID, contentText(out))
			}

			if tc.ControlArgs != nil {
				control := callConformanceTool(t, ctx, h, tc.ControlArgs)
				if control.IsError && !tc.ControlMayFail {
					t.Fatalf("claim %s negative control errored: %s", tc.ClaimID, contentText(control))
				}
				assertDiscriminatedWhenObservable(t, claim, out, control)
			}
		})
	}
}

func conformanceCases() []conformanceCase {
	return []conformanceCase{
		searchCase("phrase", `"extends PaymentProcessor"`, "extends PaymentProcessor", nil),
		searchCase("wildcard-multi", "test*", "test", nil),
		searchCase("wildcard-single", "te?t", "te*t", nil),
		searchCase("field-defs", "defs:PaymentProcessor", "PaymentProcessor", nil),
		searchCase("field-refs", "refs:processPayment", "defs:processPayment", nil),
		searchCase("field-path", "path:src", "path:no_such_path", nil),
		searchCase("field-hist", `hist:"null check"`, `"null check"`, map[string]any{"mode": "history"}),
		searchCase("field-type", "type:java", "type:no_such_type", nil),
		searchCase("boolean", "foo AND bar -baz", "foo OR bar", nil),
		searchCase("fuzzy", "paymet~", "paymet", nil),
		searchCase("proximity", `"opengrok help"~10`, `"opengrok help"~0`, nil),
		searchCase("range", "title:{Aida TO Carmen}", "title:{Zulu TO Zz}", nil),
		searchCase("regex", "/[mb]an/", "man", nil),
		searchCase("path-regex", "path:/ma[a-zA-Z]*/", "path:ma[a-zA-Z]*", nil, conditionalCase(), controlMayFail()),
		searchCase("leading-wildcard", "*Processor", "Processor", nil, conditionalCase()),
		autoQuoteCase(),
		searchCase("bare-regex", "class.*extends", "/class.*extends/", nil),
		searchCase("wildcard-in-phrase", `"foo* bar"`, "foo* bar", nil),
	}
}

type caseOption func(*conformanceCase)

func conditionalCase() caseOption {
	return func(tc *conformanceCase) { tc.Conditional = true }
}

func controlMayFail() caseOption {
	return func(tc *conformanceCase) { tc.ControlMayFail = true }
}

func wantError(code string) caseOption {
	return func(tc *conformanceCase) {
		tc.WantErrorCode = code
		tc.ControlMayFail = false
	}
}

func searchCase(claimID string, query string, controlQuery string, extra map[string]any, opts ...caseOption) conformanceCase {
	args := searchArgs(query)
	control := searchArgs(controlQuery)
	for k, v := range extra {
		args[k] = v
		control[k] = v
	}

	tc := conformanceCase{ClaimID: claimID, Args: args, ControlArgs: control}
	for _, opt := range opts {
		opt(&tc)
	}
	return tc
}

func autoQuoteCase() conformanceCase {
	args := searchArgs("extends Foo")
	control := searchArgs("extends Foo")
	control["tokenized"] = true
	return conformanceCase{ClaimID: "auto-quote", Args: args, ControlArgs: control}
}

func searchArgs(query string) map[string]any {
	return map[string]any{
		"operation":          "code",
		"query":              query,
		"allow_all_projects": true,
		"page_size":          5,
		"response_mode":      "compact",
	}
}

func callConformanceTool(t *testing.T, ctx context.Context, h *Harness, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	out, err := h.Session().CallTool(ctx, &mcp.CallToolParams{
		Name:      "opengrok_search",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if out == nil {
		t.Fatal("CallTool returned nil result")
	}
	return out
}

func assertConformanceError(t *testing.T, claimID string, out *mcp.CallToolResult, want string) {
	t.Helper()
	if !out.IsError {
		t.Fatalf("claim %s returned success, want error_code=%s", claimID, want)
	}
	body := structured(out)
	if got, _ := body["error_code"].(string); got != want {
		t.Fatalf("claim %s error_code=%q, want %q; body=%v text=%s", claimID, got, want, body, contentText(out))
	}
}

func assertDiscriminatedWhenObservable(t *testing.T, claim mcpserver.Claim, out *mcp.CallToolResult, control *mcp.CallToolResult) {
	t.Helper()
	signature := conformanceSignature(out)
	controlSignature := conformanceSignature(control)
	if signature == "" || controlSignature == "" {
		return
	}
	if signature == controlSignature {
		t.Logf("claim %s positive and control produced the same observable signature %q; live corpus may not contain discriminating data for: %s", claim.ID, signature, claim.NegativeControl)
	}
}

func conformanceSignature(out *mcp.CallToolResult) string {
	if out == nil {
		return ""
	}
	if out.IsError {
		body := structured(out)
		if code, _ := body["error_code"].(string); code != "" {
			return "error:" + code
		}
		return "error:" + strings.TrimSpace(contentText(out))
	}
	body := structured(out)
	total, ok := body["total_hits"].(float64)
	if !ok || total == 0 {
		return ""
	}
	data, err := json.Marshal(body["results"])
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func liveConformanceClaimIDs() []string {
	cases := conformanceCases()
	ids := make([]string, 0, len(cases))
	for _, tc := range cases {
		ids = append(ids, tc.ClaimID)
	}
	sort.Strings(ids)
	return ids
}
