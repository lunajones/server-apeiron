package contactpolicy

import "strings"

type Classification struct {
	Canonical         string
	AllowsPassthrough bool
	StopsAtContact    bool
	CarriesTarget     bool
	AppliesPush       bool
	GrantsIFrame      bool
}

func Classify(values ...string) Classification {
	classification := Classification{}
	for _, value := range values {
		normalized := normalize(value)
		if normalized == "" {
			continue
		}
		if classification.Canonical == "" {
			classification.Canonical = normalized
		}
		classification.merge(classifyOne(normalized))
	}
	if classification.AllowsPassthrough || classification.GrantsIFrame {
		classification.StopsAtContact = false
	}
	return classification
}

func (c *Classification) merge(other Classification) {
	if c == nil {
		return
	}
	c.AllowsPassthrough = c.AllowsPassthrough || other.AllowsPassthrough
	c.StopsAtContact = c.StopsAtContact || other.StopsAtContact
	c.CarriesTarget = c.CarriesTarget || other.CarriesTarget
	c.AppliesPush = c.AppliesPush || other.AppliesPush
	c.GrantsIFrame = c.GrantsIFrame || other.GrantsIFrame
}

func classifyOne(value string) Classification {
	classification := Classification{Canonical: value}
	if containsAny(value, "lateral_counter_contact") {
		classification.CarriesTarget = true
		return classification
	}
	if containsAny(value, "passthrough", "phase_through", "phase-through", "airborne_passthrough") {
		classification.AllowsPassthrough = true
	}
	if containsAny(value, "iframe", "invulnerable") {
		classification.GrantsIFrame = true
	}
	if containsAny(value, "carry") {
		classification.CarriesTarget = true
		classification.StopsAtContact = true
	}
	if containsAny(value, "push", "knockback") {
		classification.AppliesPush = true
		classification.StopsAtContact = true
	}
	if containsAny(value, "contact", "block") {
		classification.StopsAtContact = true
	}
	return classification
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
