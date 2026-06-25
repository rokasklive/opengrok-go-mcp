// SPDX-License-Identifier: Apache-2.0

package mcpserver

import "testing"

const enforceCompleteClaimBijection = true

var registeredClaimChecks = map[string]string{}

func init() {
	for _, claimID := range []string{
		"phrase",
		"wildcard-multi",
		"wildcard-single",
		"field-defs",
		"field-refs",
		"field-path",
		"field-hist",
		"field-type",
		"boolean",
		"fuzzy",
		"proximity",
		"range",
		"regex",
		"path-regex",
		"leading-wildcard",
		"auto-quote",
		"bare-regex",
		"wildcard-in-phrase",
	} {
		registerClaimCheck(claimID, "TestLiveConformance/"+claimID)
	}
}

func registerClaimCheck(claimID string, testRef string) {
	if claimID == "" {
		panic("claim check has empty claim ID")
	}
	if testRef == "" {
		panic("claim check has empty test ref")
	}
	if existing, ok := registeredClaimChecks[claimID]; ok && existing != testRef {
		panic("claim check registered twice with different refs: " + claimID)
	}
	registeredClaimChecks[claimID] = testRef
}

func TestClaimsHaveRequiredEvidence(t *testing.T) {
	seenIDs := map[string]bool{}
	for _, claim := range Claims() {
		if claim.ID == "" {
			t.Error("claim has empty ID")
		}
		if seenIDs[claim.ID] {
			t.Errorf("claim ID %q is duplicated", claim.ID)
		}
		seenIDs[claim.ID] = true

		if claim.Category == "" {
			t.Errorf("%s: category is empty", claim.ID)
		}
		if claim.SupportStatus == "" {
			t.Errorf("%s: support status is empty", claim.ID)
		}
		if claim.AgentClaimText == "" {
			t.Errorf("%s: agent claim text is empty", claim.ID)
		}
		if len(claim.AppliesTo) == 0 {
			t.Errorf("%s: applies_to is empty", claim.ID)
		}
		if claim.DisclosureLocation == "" {
			t.Errorf("%s: disclosure location is empty", claim.ID)
		}
		if claim.GroundTruthSource == "" {
			t.Errorf("%s: ground truth source is empty", claim.ID)
		}
		if claim.SupportStatus == ClaimSupportConditional && claim.Condition == "" {
			t.Errorf("%s: conditional claim lacks condition", claim.ID)
		}

		if claim.Gate == ClaimGateNone {
			if claim.NoneGateJustification == "" {
				t.Errorf("%s: none-gate claim lacks justification", claim.ID)
			}
			continue
		}
		if claim.ConformanceTestRef == "" {
			t.Errorf("%s: conformance test ref is empty", claim.ID)
		}
		if claim.PositiveAssertion == "" {
			t.Errorf("%s: positive assertion is empty", claim.ID)
		}
		if claim.NegativeControl == "" {
			t.Errorf("%s: negative control is empty", claim.ID)
		}
	}
}

func TestRegisteredClaimChecksResolve(t *testing.T) {
	for claimID, testRef := range registeredClaimChecks {
		claim, ok := ClaimByID(claimID)
		if !ok {
			t.Errorf("registered claim check %q references unknown claim %q", testRef, claimID)
			continue
		}
		if claim.ConformanceTestRef != testRef {
			t.Errorf(
				"%s: registered test ref = %q, claim conformance_test_ref = %q",
				claimID,
				testRef,
				claim.ConformanceTestRef,
			)
		}
	}
}

func TestCompleteClaimCheckBijection(t *testing.T) {
	if !enforceCompleteClaimBijection {
		t.Skip("complete claim-check bijection is enabled after live and always-on checks register")
	}

	for _, claim := range Claims() {
		if claim.Gate == ClaimGateNone {
			continue
		}
		testRef, ok := registeredClaimChecks[claim.ID]
		if !ok {
			t.Errorf("%s: claim has no registered check", claim.ID)
			continue
		}
		if testRef != claim.ConformanceTestRef {
			t.Errorf(
				"%s: registered test ref = %q, claim conformance_test_ref = %q",
				claim.ID,
				testRef,
				claim.ConformanceTestRef,
			)
		}
	}
}
