package state

import (
	"strconv"
	"strings"
	"testing"
)

func TestParseForwardReferencesAndPseudoStates(t *testing.T) {
	d, err := Parse("stateDiagram-v2\ndirection LR\nA --> B : go [ready]\n[*] --> A\nB --> [*]\nstate A\nstate \"Bee"+" label\" as B\nstate Idle\n", DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if d.Direction != LeftRight || len(d.States) != 3 || len(d.Transitions) != 3 {
		t.Fatalf("unexpected diagram: %#v", d)
	}
	if d.Transitions[0].Label() != "go [ready]" || d.Transitions[0].From.Index != 0 || d.Transitions[0].To.Index != 1 {
		t.Fatalf("forward reference not resolved: %#v", d.Transitions[0])
	}
}

func TestParseAliasLabelMayContainArrowText(t *testing.T) {
	diagram, err := Parse("stateDiagram-v2\n[*] --> A\nstate \"before --> after\" as A\n", DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if diagram.States[0].Label != "before --> after" {
		t.Fatalf("label=%q", diagram.States[0].Label)
	}
}

func TestParseRejectsRequiredStateInvariants(t *testing.T) {
	for _, source := range []string{
		"stateDiagram-v2\nstate A\nA --> A\n",
		"stateDiagram-v2\nstate A\n[*] --> A\n[*] --> A\n",
		"stateDiagram-v2\nstate A\n[*] --> A : no\n",
		"stateDiagram-v2\nstate A\n[*]--> A\n",
	} {
		if _, err := Parse(source, DefaultLimits()); err == nil {
			t.Fatalf("expected rejection for %q", source)
		}
	}
}

func TestParseRejectsInvalidLimitsAndReportsUnsafeCharacterLocation(t *testing.T) {
	if _, err := Parse("stateDiagram-v2\n", Limits{}); err == nil {
		t.Fatal("zero limits accepted")
	}
	if _, err := Parse("stateDiagram-v2\n\u200d", DefaultLimits()); err == nil {
		t.Fatal("ZWJ accepted")
	} else if got := err.(*ParseError); got.Line != 2 || got.Column != 1 {
		t.Fatalf("location=%+v", got)
	}
	if _, err := Parse("stateDiagram-v2\rstate A\n", DefaultLimits()); err == nil {
		t.Fatal("lone CR accepted")
	}
}

func TestParseLimitsAndSemanticEdges(t *testing.T) {
	base := "stateDiagram-v2\n[*] --> A\nstate A\n"
	if _, err := Parse(base, DefaultLimits()); err != nil {
		t.Fatalf("final state should be optional: %v", err)
	}
	for _, source := range []string{
		"stateDiagram-v2\n[*] --> Missing\nstate A\n",
		"stateDiagram-v2\n[*] --> A\nstate \"same\" as A\nstate \"same\" as B\n",
		"stateDiagram-v2\n[*] --> A\nA --> B : x\nA --> B : x\nstate A\nstate B\n",
	} {
		if _, err := Parse(source, DefaultLimits()); err == nil {
			t.Fatalf("expected semantic rejection: %q", source)
		}
	}
	limits := DefaultLimits()
	limits.MaxBytes = 1 << 30
	limits.MaxLines = 1 << 30
	if _, err := Parse(base, limits); err != nil {
		t.Fatalf("huge limits must not alter hard parser behavior: %v", err)
	}
	label96 := strings.Repeat("x", 96)
	if _, err := Parse("stateDiagram-v2\n[*] --> A\nstate \""+label96+"\" as A\n", DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse("stateDiagram-v2\n[*] --> A\nstate \""+strings.Repeat("x", 97)+"\" as A\n", DefaultLimits()); err == nil {
		t.Fatal("97-cell label accepted")
	}
}

func TestParseHardStateAndTransitionLimits(t *testing.T) {
	var states strings.Builder
	states.WriteString("stateDiagram-v2\n[*] --> S0\n")
	for i := 0; i < 32; i++ {
		states.WriteString("state S" + strconv.Itoa(i) + "\n")
	}
	if _, err := Parse(states.String(), DefaultLimits()); err != nil {
		t.Fatalf("32 states rejected: %v", err)
	}
	states.WriteString("state S32\n")
	if _, err := Parse(states.String(), DefaultLimits()); err == nil {
		t.Fatal("33 states accepted")
	}
	var transitions strings.Builder
	transitions.WriteString("stateDiagram-v2\n[*] --> A\nstate A\nstate B\n")
	for i := 0; i < 63; i++ {
		transitions.WriteString("A --> B : e" + strconv.Itoa(i) + "\n")
	}
	if _, err := Parse(transitions.String(), DefaultLimits()); err != nil {
		t.Fatalf("64 transitions rejected: %v", err)
	}
	transitions.WriteString("A --> B : overflow\n")
	if _, err := Parse(transitions.String(), DefaultLimits()); err == nil {
		t.Fatal("65 transitions accepted")
	}
}

func TestParseTransitionLabelCellBoundary(t *testing.T) {
	event96 := strings.Repeat("e", 96)
	if _, err := Parse("stateDiagram-v2\n[*] --> A\nA --> B : "+event96+"\nstate A\nstate B\n", DefaultLimits()); err != nil {
		t.Fatalf("96-cell event rejected: %v", err)
	}
	if _, err := Parse("stateDiagram-v2\n[*] --> A\nA --> B : "+event96+"e\nstate A\nstate B\n", DefaultLimits()); err == nil {
		t.Fatal("97-cell event accepted")
	}
	guard := strings.Repeat("g", 92)
	if _, err := Parse("stateDiagram-v2\n[*] --> A\nA --> B : e ["+guard+"]\nstate A\nstate B\n", DefaultLimits()); err != nil {
		t.Fatalf("96-cell event+guard rejected: %v", err)
	}
}

func TestParseMalformedStateGrammar(t *testing.T) {
	for _, source := range []string{
		"stateDiagram-v2X\n[*] --> A\nstate A\n",
		"stateDiagram-v2 LR\n[*] --> A\nstate A\n",
		"stateDiagram-v2\ndirection LR\ndirection TD\n[*] --> A\nstate A\n",
		"stateDiagram-v2\nstate A\ndirection LR\n[*] --> A\n",
		"stateDiagram-v2\ndirection tb\n[*] --> A\nstate A\n",
		"stateDiagram-v2\nstate \"a\"b\" as A\n[*] --> A\n",
		"stateDiagram-v2\n[*] --> [*]\nstate A\n",
		"stateDiagram-v2\nA --> [*] : done\n[*] --> A\nstate A\n",
		"stateDiagram-v2\n[*] --> A\nA --> B : \nstate A\nstate B\n",
		"stateDiagram-v2\n[*] --> A\nA --> B : [guard]\nstate A\nstate B\n",
		"stateDiagram-v2\n[*] --> A\nA --> B : x [a] [b]\nstate A\nstate B\n",
	} {
		if _, err := Parse(source, DefaultLimits()); err == nil {
			t.Fatalf("accepted malformed source: %q", source)
		}
	}
}

func TestParseIDAndCRLFBounds(t *testing.T) {
	id64 := "A" + strings.Repeat("a", 63)
	if _, err := Parse("stateDiagram-v2\r\n[*] --> "+id64+"\r\nstate "+id64+"\r\n", DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	id65 := id64 + "a"
	if _, err := Parse("stateDiagram-v2\n[*] --> "+id65+"\nstate "+id65+"\n", DefaultLimits()); err == nil {
		t.Fatal("65-byte ID accepted")
	}
	limits := DefaultLimits()
	limits.MaxLines = 2
	if _, err := Parse("stateDiagram-v2\n[*] --> A\nstate A\n", limits); err == nil {
		t.Fatal("line cap ignored")
	}
	for _, bad := range []string{"\u00a0", "\u202e", "\u200d", "\ufe0f"} {
		if _, err := Parse("stateDiagram-v2\n"+bad, DefaultLimits()); err == nil {
			t.Fatalf("unsafe rune accepted: %q", bad)
		}
	}
}

func TestParseRawByteLimitAppliesBeforeCRLFNormalization(t *testing.T) {
	source := "stateDiagram-v2\r\n[*] --> A\r\nstate A\r\n"
	normalized := strings.ReplaceAll(source, "\r\n", "\n")
	limits := DefaultLimits()
	limits.MaxBytes = len(normalized)
	if _, err := Parse(source, limits); err == nil {
		t.Fatal("CRLF normalization bypassed raw byte limit")
	}
	limits = DefaultLimits()
	limits.MaxBytes = 16
	if _, err := Parse(strings.Repeat("x", 17), limits); err == nil || !strings.Contains(err.Error(), "입력 크기 제한 초과") {
		t.Fatalf("oversized ASCII did not hit early size gate: %v", err)
	}
}

func TestParseTransitionPoliciesResolveExactTransitions(t *testing.T) {
	source := `stateDiagram-v2
policy Backoff --> Committing : retry :: retry "attempt below 3"
policy Waiting --> TimedOut : deadline elapsed :: timeout "30s from enqueue"
policy Committed --> Compensating : cancel accepted :: compensation "release reservation"
[*] --> Committing
Committing --> Backoff : transient failure [attempt below 3]
Backoff --> Committing : retry
Waiting --> TimedOut : deadline elapsed
Committed --> Compensating : cancel accepted
state Committing
state Backoff
state Waiting
state TimedOut
state Committed
state Compensating
`
	diagram, err := Parse(source, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	want := []TransitionPolicy{
		{TransitionIndex: 2, Kind: RetryPolicy, Detail: "attempt below 3"},
		{TransitionIndex: 3, Kind: TimeoutPolicy, Detail: "30s from enqueue"},
		{TransitionIndex: 4, Kind: CompensationPolicy, Detail: "release reservation"},
	}
	if len(diagram.Policies) != len(want) {
		t.Fatalf("policies=%#v", diagram.Policies)
	}
	for index := range want {
		if diagram.Policies[index] != want[index] {
			t.Fatalf("policy[%d]=%#v want %#v", index, diagram.Policies[index], want[index])
		}
	}
}

func TestParseDoesNotPromotePolicyWordsInEventLabels(t *testing.T) {
	diagram, err := Parse("stateDiagram-v2\n[*] --> A\nA --> B : retry timeout compensate\nstate A\nstate B\n", DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if len(diagram.Policies) != 0 {
		t.Fatalf("event words promoted to policies: %#v", diagram.Policies)
	}
}

func TestParsePolicySeparatorDoesNotReinterpretEventOrDetail(t *testing.T) {
	source := "stateDiagram-v2\n[*] --> A\nA --> B : prepare :: retry\nstate A\nstate B\npolicy A --> B : prepare :: retry :: timeout \"30s :: from enqueue\"\n"
	diagram, err := Parse(source, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if len(diagram.Policies) != 1 || diagram.Policies[0].Kind != TimeoutPolicy || diagram.Policies[0].Detail != "30s :: from enqueue" {
		t.Fatalf("policy=%#v", diagram.Policies)
	}
}

func TestParseRejectsQuotedPolicyTargetWithoutBreakingQuotedEvents(t *testing.T) {
	quotedEvent := "stateDiagram-v2\n[*] --> A\nA --> B : go \"quoted\"\nstate A\nstate B\n"
	if _, err := Parse(quotedEvent, DefaultLimits()); err != nil {
		t.Fatalf("quoted ordinary event lost backward compatibility: %v", err)
	}
	for _, source := range []string{
		"stateDiagram-v2\n[*] --> A\nA --> B : evt :: retry \"detail\nstate A\nstate B\npolicy A --> B : evt :: retry \"detail :: timeout \"x\"\n",
		quotedEvent + "policy A --> B : go \"quoted\" :: retry \"detail\"\n",
		"stateDiagram-v2\n[*] --> A\nA --> B : go [\"ready\"]\nstate A\nstate B\npolicy A --> B : go [\"ready\"] :: retry \"detail\"\n",
	} {
		if _, err := Parse(source, DefaultLimits()); err == nil {
			t.Fatalf("quoted policy target accepted: %q", source)
		}
	}
}

func TestParseRejectsMalformedOrUnresolvedPolicies(t *testing.T) {
	base := "stateDiagram-v2\n[*] --> A\nA --> B : go [ready]\nstate A\nstate B\n"
	for _, policy := range []string{
		"policy A --> B : go [ready] :: unknown \"detail\"",
		"policy A --> B : go [ready] :: retry \"\"",
		"policy A --> B : go [ready] :: retry \"",
		"policy A --> B : go [ready] :: retry detail",
		"policy A --> B : go [ready] :: retry \"bad \" quote\"",
		"policy A --> B : go :: retry \"mismatched guard\"",
		"policy A --> B : missing [ready] :: retry \"mismatched event\"",
		"policy [*] --> A : boot :: retry \"pseudo\"",
		"policy A --> B :: retry \"unlabeled\"",
	} {
		if _, err := Parse(base+policy+"\n", DefaultLimits()); err == nil {
			t.Fatalf("accepted invalid policy: %q", policy)
		}
	}
	duplicate := base +
		"policy A --> B : go [ready] :: retry \"first\"\n" +
		"policy A --> B : go [ready] :: retry \"second\"\n"
	if _, err := Parse(duplicate, DefaultLimits()); err == nil {
		t.Fatal("duplicate transition policy accepted")
	}
	multipleKinds := base +
		"policy A --> B : go [ready] :: retry \"attempt below 3\"\n" +
		"policy A --> B : go [ready] :: timeout \"request deadline\"\n"
	if _, err := Parse(multipleKinds, DefaultLimits()); err != nil {
		t.Fatalf("different policy kinds on one transition rejected: %v", err)
	}
}

func TestParsePolicyLimitsAndDetailCellBoundary(t *testing.T) {
	var source strings.Builder
	source.WriteString("stateDiagram-v2\n[*] --> A\nstate A\nstate B\n")
	for index := 0; index < 22; index++ {
		source.WriteString("A --> B : e" + strconv.Itoa(index) + "\n")
	}
	kinds := []string{"retry", "timeout", "compensation"}
	for index := 0; index < 64; index++ {
		source.WriteString("policy A --> B : e" + strconv.Itoa(index%22) + " :: " + kinds[index%3] + " \"d" + strconv.Itoa(index) + "\"\n")
	}
	if _, err := Parse(source.String(), DefaultLimits()); err != nil {
		t.Fatalf("64 policies rejected: %v", err)
	}
	source.WriteString("policy A --> B : e20 :: timeout \"overflow\"\n")
	if _, err := Parse(source.String(), DefaultLimits()); err == nil {
		t.Fatal("65 policies accepted")
	}
	detail96 := strings.Repeat("d", 96)
	boundary := "stateDiagram-v2\n[*] --> A\nA --> B : go\nstate A\nstate B\npolicy A --> B : go :: retry \"" + detail96 + "\"\n"
	if _, err := Parse(boundary, DefaultLimits()); err != nil {
		t.Fatalf("96-cell policy detail rejected: %v", err)
	}
	if _, err := Parse(strings.Replace(boundary, detail96, detail96+"d", 1), DefaultLimits()); err == nil {
		t.Fatal("97-cell policy detail accepted")
	}
}
