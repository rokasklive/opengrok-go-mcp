// SPDX-License-Identifier: Apache-2.0

package mcpserver

// ClaimCategory identifies the kind of capability claim.
type ClaimCategory string

const (
	ClaimCategoryQuerySyntax         ClaimCategory = "query-syntax"
	ClaimCategoryBehavioralGuarantee ClaimCategory = "behavioral-guarantee"
	ClaimCategoryLimitation          ClaimCategory = "limitation"
)

// ClaimSupportStatus records whether a claim is supported, unsupported, or
// deployment-conditional.
type ClaimSupportStatus string

const (
	ClaimSupportSupported   ClaimSupportStatus = "supported"
	ClaimSupportUnsupported ClaimSupportStatus = "unsupported"
	ClaimSupportConditional ClaimSupportStatus = "conditional"
)

// ClaimDisclosureLocation records where a claim is disclosed to agents.
type ClaimDisclosureLocation string

const (
	ClaimDisclosureInlineDescription    ClaimDisclosureLocation = "inline-description"
	ClaimDisclosureCapabilitiesManifest ClaimDisclosureLocation = "capabilities-manifest"
)

// ClaimGate identifies which suite verifies a claim.
type ClaimGate string

const (
	ClaimGateLive     ClaimGate = "live"
	ClaimGateAlwaysOn ClaimGate = "always-on"
	ClaimGateNone     ClaimGate = "none"
)

// Claim is one row in the claim registry. It is the single source of truth for
// agent-visible capability prose and the conformance matrix.
type Claim struct {
	ID                    string
	Category              ClaimCategory
	SupportStatus         ClaimSupportStatus
	Condition             string
	AgentClaimText        string
	Example               string
	AppliesTo             []string
	DisclosureLocation    ClaimDisclosureLocation
	GroundTruthSource     string
	ConformanceTestRef    string
	PositiveAssertion     string
	NegativeControl       string
	Gate                  ClaimGate
	NoneGateJustification string
}

var claimRegistry = []Claim{
	{
		ID:                    "opengrok-nature",
		Category:              ClaimCategoryBehavioralGuarantee,
		SupportStatus:         ClaimSupportSupported,
		AgentClaimText:        "OpenGrok is full-text search plus ctags symbols, not an AST/call-graph/type-hierarchy engine.",
		AppliesTo:             []string{"opengrok_projects", "opengrok_search", "opengrok_symbols", "opengrok_read", "search_code", "search_and_read", "search_symbol_definitions", "search_symbol_references", "search_implementations"},
		DisclosureLocation:    ClaimDisclosureInlineDescription,
		GroundTruthSource:     "docs/limitations.md#OpenGrok Semantics And Limitations",
		Gate:                  ClaimGateNone,
		NoneGateJustification: "architectural boundary documented for agent planning; no single executable OpenGrok behavior proves the full boundary",
	},
	{
		ID:                 "phrase",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     `Wrap multi-word queries in double quotes for an exact phrase.`,
		Example:            `"extends PaymentProcessor"`,
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureInlineDescription,
		GroundTruthSource:  "help-syntax.snapshot.md#query-clauses; #examples",
		ConformanceTestRef: "TestLiveConformance/phrase",
		PositiveAssertion:  "exact adjacent phrase results are distinguishable from bag-of-words results",
		NegativeControl:    "tokenized bag-of-words control returns a looser or different result set",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "wildcard-multi",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "`*` matches multiple characters.",
		Example:            "test*",
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureCapabilitiesManifest,
		GroundTruthSource:  "help-syntax.snapshot.md#regex-wildcard-fuzzy-proximity-and-range-searches",
		ConformanceTestRef: "TestLiveConformance/wildcard-multi",
		PositiveAssertion:  "multi-character wildcard expands to matching terms",
		NegativeControl:    "exact-term control is distinguishable from wildcard expansion",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "wildcard-single",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "`?` matches a single character.",
		Example:            "te?t",
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureCapabilitiesManifest,
		GroundTruthSource:  "help-syntax.snapshot.md#regex-wildcard-fuzzy-proximity-and-range-searches",
		ConformanceTestRef: "TestLiveConformance/wildcard-single",
		PositiveAssertion:  "single-character wildcard expands to matching terms",
		NegativeControl:    "multi-character wildcard control is distinguishable",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "field-defs",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "`defs:Name` finds symbol definitions.",
		Example:            "defs:PaymentProcessor",
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read", "opengrok_symbols"},
		DisclosureLocation: ClaimDisclosureInlineDescription,
		GroundTruthSource:  "help-syntax.snapshot.md#valid-fields; #examples",
		ConformanceTestRef: "TestLiveConformance/field-defs",
		PositiveAssertion:  "definition field returns definition hits",
		NegativeControl:    "plain full-text control returns non-definition hits or a different set",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "field-refs",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "`refs:Name` finds symbol references.",
		Example:            "refs:processPayment",
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read", "opengrok_symbols"},
		DisclosureLocation: ClaimDisclosureInlineDescription,
		GroundTruthSource:  "help-syntax.snapshot.md#valid-fields; #examples",
		ConformanceTestRef: "TestLiveConformance/field-refs",
		PositiveAssertion:  "reference field returns reference hits",
		NegativeControl:    "definition-only control is distinguishable",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "field-path",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "`path:` restricts matches by source-file path.",
		Example:            "path:src/api",
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureInlineDescription,
		GroundTruthSource:  "help-syntax.snapshot.md#valid-fields; #examples",
		ConformanceTestRef: "TestLiveConformance/field-path",
		PositiveAssertion:  "path field restricts results to matching paths",
		NegativeControl:    "out-of-path control is excluded or distinguishable",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "field-hist",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "`hist:` searches history log comments in history mode.",
		Example:            `hist:"null check"`,
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureCapabilitiesManifest,
		GroundTruthSource:  "help-syntax.snapshot.md#valid-fields",
		ConformanceTestRef: "TestLiveConformance/field-hist",
		PositiveAssertion:  "history field is honored in history searches",
		NegativeControl:    "non-history mode is ignored or warned as mode-sensitive",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "field-type",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "`type:` filters by analyzer/file type.",
		Example:            "type:java",
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureCapabilitiesManifest,
		GroundTruthSource:  "help-syntax.snapshot.md#valid-fields; #examples",
		ConformanceTestRef: "TestLiveConformance/field-type",
		PositiveAssertion:  "file-type field restricts results to the requested type",
		NegativeControl:    "different type control is excluded or distinguishable",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "boolean",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "`AND`, `OR`, `NOT`, `+`, and `-` combine or require/prohibit terms.",
		Example:            "foo AND bar -baz",
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureCapabilitiesManifest,
		GroundTruthSource:  "help-syntax.snapshot.md#query-clauses",
		ConformanceTestRef: "TestLiveConformance/boolean",
		PositiveAssertion:  "required/prohibited terms affect the result set",
		NegativeControl:    "OR or omitted-prohibition control is distinguishable",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "fuzzy",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "`~` does fuzzy edit-distance matching.",
		Example:            "paymet~",
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureCapabilitiesManifest,
		GroundTruthSource:  "help-syntax.snapshot.md#regex-wildcard-fuzzy-proximity-and-range-searches",
		ConformanceTestRef: "TestLiveConformance/fuzzy",
		PositiveAssertion:  "fuzzy form matches a nearby spelling",
		NegativeControl:    "exact typo control misses or differs",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "proximity",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "`\"a b\"~N` matches words within N positions.",
		Example:            `"opengrok help"~10`,
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureCapabilitiesManifest,
		GroundTruthSource:  "help-syntax.snapshot.md#regex-wildcard-fuzzy-proximity-and-range-searches",
		ConformanceTestRef: "TestLiveConformance/proximity",
		PositiveAssertion:  "proximity form matches near co-occurrence",
		NegativeControl:    "stricter proximity control is distinguishable",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "range",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "`[A TO B]` and `{A TO B}` express inclusive/exclusive ranges.",
		Example:            "title:{Aida TO Carmen}",
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureCapabilitiesManifest,
		GroundTruthSource:  "help-syntax.snapshot.md#regex-wildcard-fuzzy-proximity-and-range-searches",
		ConformanceTestRef: "TestLiveConformance/range",
		PositiveAssertion:  "range form restricts lexicographic bounds",
		NegativeControl:    "out-of-range control is excluded or distinguishable",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "regex",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "Regex is supported only with `/.../` enclosure.",
		Example:            "/[mb]an/",
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureInlineDescription,
		GroundTruthSource:  "help-syntax.snapshot.md#regex-wildcard-fuzzy-proximity-and-range-searches",
		ConformanceTestRef: "TestLiveConformance/regex",
		PositiveAssertion:  "slash-enclosed regex matches regex alternatives",
		NegativeControl:    "literal control is distinguishable from regex",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "path-regex",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportConditional,
		Condition:          "`path:` treats regex as regex only when the value starts and ends with `/`.",
		AgentClaimText:     "`path:` honors regex when the path value starts and ends with `/`.",
		Example:            "path:/ma[a-zA-Z]*/",
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureCapabilitiesManifest,
		GroundTruthSource:  "help-syntax.snapshot.md#regex-wildcard-fuzzy-proximity-and-range-searches; #examples",
		ConformanceTestRef: "TestLiveConformance/path-regex",
		PositiveAssertion:  "slash-delimited path regex matches paths by regex",
		NegativeControl:    "unslashed path control is treated literally or differs",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "leading-wildcard",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportConditional,
		Condition:          "Leading wildcards depend on the OpenGrok indexer not disabling them with `-a`.",
		AgentClaimText:     "Leading `*` and `?` may be accepted unless the indexer disabled them.",
		Example:            "*Processor",
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureCapabilitiesManifest,
		GroundTruthSource:  "help-syntax.snapshot.md#regex-wildcard-fuzzy-proximity-and-range-searches",
		ConformanceTestRef: "TestLiveConformance/leading-wildcard",
		PositiveAssertion:  "leading wildcard matches suffix terms when enabled",
		NegativeControl:    "non-leading control is distinguishable or skipped when disabled",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "auto-quote",
		Category:           ClaimCategoryBehavioralGuarantee,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "Bare multi-word queries are auto-quoted as exact phrases unless `tokenized=true` is set.",
		Example:            "extends Foo",
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureInlineDescription,
		GroundTruthSource:  "internal/mcpserver/query.go; docs/tool-contracts.md#query-handling",
		ConformanceTestRef: "TestLiveConformance/auto-quote",
		PositiveAssertion:  "bare multi-word query sent by the MCP surface is phrase-searched by default",
		NegativeControl:    "`tokenized=true` control searches terms independently",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "projects-array",
		Category:           ClaimCategoryBehavioralGuarantee,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "`projects:[...]` scopes searches to multiple explicit projects.",
		Example:            `projects:["platform","infra"]`,
		AppliesTo:          []string{"opengrok_search", "opengrok_symbols", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureCapabilitiesManifest,
		GroundTruthSource:  "internal/mcpserver/types.go SearchCodeInput.Projects",
		ConformanceTestRef: "TestProjectsArrayAcceptedAndApplied",
		PositiveAssertion:  "projects array validates and is forwarded to backend scoping",
		NegativeControl:    "single project control forwards a single-project scope",
		Gate:               ClaimGateAlwaysOn,
	},
	{
		ID:                 "scalar-coercion",
		Category:           ClaimCategoryBehavioralGuarantee,
		SupportStatus:      ClaimSupportSupported,
		AgentClaimText:     "String-encoded scalar arguments are coerced before schema validation.",
		Example:            `before:"10"`,
		AppliesTo:          []string{"opengrok_projects", "opengrok_search", "opengrok_symbols", "opengrok_read", "list_files", "get_project_overview", "search_code", "get_file_context"},
		DisclosureLocation: ClaimDisclosureCapabilitiesManifest,
		GroundTruthSource:  "internal/mcpserver/coerce.go scalarCoercer",
		ConformanceTestRef: "TestScalarCoercionStringEncodedBeforeValidation",
		PositiveAssertion:  "string-encoded scalar validates and reaches the handler as its declared type",
		NegativeControl:    "unparseable scalar string remains rejected by validation",
		Gate:               ClaimGateAlwaysOn,
	},
	{
		ID:             "default-project",
		Category:       ClaimCategoryBehavioralGuarantee,
		SupportStatus:  ClaimSupportSupported,
		AgentClaimText: "Omitting `project` uses the configured default project when one is resolved.",
		// No inline Example: this behavioral claim applies to tools with disjoint
		// operation schemas, so a single per-tool JSON payload cannot be valid for
		// all of them. Each tool renders its own schema-valid example instead.
		AppliesTo:          []string{"opengrok_search", "opengrok_symbols", "opengrok_read", "search_code", "get_file_context"},
		DisclosureLocation: ClaimDisclosureInlineDescription,
		GroundTruthSource:  "internal/mcpserver/helpers.go resolveProjects",
		ConformanceTestRef: "TestDefaultProjectOmittedUsesConfiguredDefault",
		PositiveAssertion:  "omitted project resolves to the configured default without discovery",
		NegativeControl:    "explicit project control overrides the default",
		Gate:               ClaimGateAlwaysOn,
	},
	{
		ID:                 "bare-regex",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportUnsupported,
		AgentClaimText:     "Bare regex is not regex; wrap regex in `/.../`.",
		Example:            "class.*extends",
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureInlineDescription,
		GroundTruthSource:  "help-syntax.snapshot.md#regex-wildcard-fuzzy-proximity-and-range-searches",
		ConformanceTestRef: "TestLiveConformance/bare-regex",
		PositiveAssertion:  "bare regex-like query is rejected or not treated as regex",
		NegativeControl:    "slash-enclosed regex control is accepted as regex",
		Gate:               ClaimGateLive,
	},
	{
		ID:                 "wildcard-in-phrase",
		Category:           ClaimCategoryQuerySyntax,
		SupportStatus:      ClaimSupportUnsupported,
		AgentClaimText:     "Wildcards inside quoted phrases do not expand.",
		Example:            `"foo* bar"`,
		AppliesTo:          []string{"opengrok_search", "search_code", "search_and_read"},
		DisclosureLocation: ClaimDisclosureInlineDescription,
		GroundTruthSource:  "help-syntax.snapshot.md#query-clauses; #regex-wildcard-fuzzy-proximity-and-range-searches",
		ConformanceTestRef: "TestLiveConformance/wildcard-in-phrase",
		PositiveAssertion:  "quoted phrase treats wildcard characters literally",
		NegativeControl:    "unquoted wildcard control expands or differs",
		Gate:               ClaimGateLive,
	},
	{
		ID:                    "inheritance",
		Category:              ClaimCategoryLimitation,
		SupportStatus:         ClaimSupportUnsupported,
		AgentClaimText:        "OpenGrok has no AST/type-hierarchy/subclass query; approximate with text and ctags searches.",
		AppliesTo:             []string{"opengrok_search", "opengrok_symbols", "search_implementations"},
		DisclosureLocation:    ClaimDisclosureInlineDescription,
		GroundTruthSource:     "docs/limitations.md#OpenGrok Semantics And Limitations",
		Gate:                  ClaimGateNone,
		NoneGateJustification: "negative limitation claim; no positive OpenGrok behavior exists to assert",
	},
	{
		ID:                    "call-graph",
		Category:              ClaimCategoryLimitation,
		SupportStatus:         ClaimSupportUnsupported,
		AgentClaimText:        "OpenGrok has no call-graph/caller query; `refs:` is ctags-backed symbol text, not semantic call analysis.",
		AppliesTo:             []string{"opengrok_search", "opengrok_symbols", "search_implementations"},
		DisclosureLocation:    ClaimDisclosureInlineDescription,
		GroundTruthSource:     "docs/limitations.md#OpenGrok Semantics And Limitations",
		Gate:                  ClaimGateNone,
		NoneGateJustification: "negative limitation claim; no positive OpenGrok behavior exists to assert",
	},
}

// Claims returns a defensive copy of all claim registry entries.
func Claims() []Claim {
	out := make([]Claim, len(claimRegistry))
	for i, claim := range claimRegistry {
		out[i] = cloneClaim(claim)
	}
	return out
}

// ClaimByID returns a claim by stable claim ID.
func ClaimByID(id string) (Claim, bool) {
	for _, claim := range claimRegistry {
		if claim.ID == id {
			return cloneClaim(claim), true
		}
	}
	return Claim{}, false
}

// ClaimsForTool returns claims that apply to the given tool or operation name.
func ClaimsForTool(tool string) []Claim {
	var out []Claim
	for _, claim := range claimRegistry {
		for _, appliesTo := range claim.AppliesTo {
			if appliesTo == tool {
				out = append(out, cloneClaim(claim))
				break
			}
		}
	}
	return out
}

// ClaimsByGate returns claims verified by the given gate.
func ClaimsByGate(gate ClaimGate) []Claim {
	var out []Claim
	for _, claim := range claimRegistry {
		if claim.Gate == gate {
			out = append(out, cloneClaim(claim))
		}
	}
	return out
}

func cloneClaim(claim Claim) Claim {
	claim.AppliesTo = append([]string(nil), claim.AppliesTo...)
	return claim
}
