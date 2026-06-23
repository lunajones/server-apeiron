package ai

import (
	"strings"
	"testing"
)

func TestValidatePolicyAcceptsCompleteCreatureBrainPolicy(t *testing.T) {
	if issues := ValidatePolicy(testPolicy()); len(issues) != 0 {
		t.Fatalf("complete policy issues = %#v", issues)
	}
}

func TestValidatePolicyRejectsMissingThreatWeightsAndBrokenBindings(t *testing.T) {
	policy := testPolicy()
	policy.CommitThreatWeight = 0
	policy.VulnerableBiteMultiplier = 0
	policy.Bindings[0].SetupPolicyID = "missing_setup"
	policy.Bindings[1].Priority = 0

	issues := ValidatePolicy(policy)
	for _, want := range []string{
		"commit threat weight",
		"vulnerable bite multiplier",
		"binding bind_lunge_circle setup policy missing",
		"binding bind_maul_pressure priority",
	} {
		if !containsIssue(issues, want) {
			t.Fatalf("issues %#v missing %q", issues, want)
		}
	}
}

func containsIssue(issues []string, want string) bool {
	for _, issue := range issues {
		if strings.Contains(issue, want) {
			return true
		}
	}
	return false
}
