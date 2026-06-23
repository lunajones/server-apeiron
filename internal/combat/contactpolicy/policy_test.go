package contactpolicy

import "testing"

func TestClassifyAllowsAirbornePassthroughWithoutStoppingAtContact(t *testing.T) {
	classification := Classify("airborne_passthrough", "melee_contact")
	if !classification.AllowsPassthrough {
		t.Fatalf("passthrough not detected: %#v", classification)
	}
	if classification.StopsAtContact {
		t.Fatalf("passthrough should override stop-at-contact: %#v", classification)
	}
}

func TestClassifyCarryPushStopsAndMarksContactResponse(t *testing.T) {
	classification := Classify("multi_target_carry_push")
	if !classification.StopsAtContact {
		t.Fatalf("carry push should stop at contact: %#v", classification)
	}
	if !classification.CarriesTarget {
		t.Fatalf("carry not detected: %#v", classification)
	}
	if !classification.AppliesPush {
		t.Fatalf("push not detected: %#v", classification)
	}
}

func TestClassifyIFrameDoesNotBecomeContactStop(t *testing.T) {
	classification := Classify("iframe")
	if !classification.GrantsIFrame {
		t.Fatalf("iframe not detected: %#v", classification)
	}
	if classification.StopsAtContact {
		t.Fatalf("iframe should not stop at contact: %#v", classification)
	}
}
