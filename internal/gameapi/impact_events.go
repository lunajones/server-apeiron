package gameapi

import (
	"fmt"
	"strconv"
	"strings"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
)

func (r *Runtime) damageEventsFromImpactsLocked(impacts []runtimeSkillImpact) []*gamev1.SnapshotEvent {
	if r == nil || len(impacts) == 0 {
		return nil
	}
	events := make([]*gamev1.SnapshotEvent, 0, len(impacts))
	for index, impact := range impacts {
		if impact.SourceID == 0 || impact.TargetID == 0 || impact.SkillID == "" {
			continue
		}
		source := r.entities[impact.SourceID]
		target := r.entities[impact.TargetID]
		impactType := strings.TrimSpace(impact.ImpactType)
		if impactType == "" {
			impactType = "physical"
		}
		responseProfile := strings.TrimSpace(impact.ImpactResponseProfile)
		if responseProfile == "" {
			responseProfile = impactProfileForEntity(target)
		}
		metadata := map[string]string{
			"skill_id":                 impact.SkillID,
			"damage_type":              "physical",
			"impact_type":              impactType,
			"impact_response_profile":  responseProfile,
			"target_response_profile":  responseProfile,
			"source_impact_profile":    impactProfileForEntity(source),
			"target_impact_profile":    impactProfileForEntity(target),
			"damage":                   formatImpactFloat(impact.DamageApplied),
			"damage_amount":            formatImpactFloat(impact.DamageApplied),
			"posture_damage":           formatImpactFloat(impact.PostureApplied),
			"blocked":                  strconv.FormatBool(impact.Blocked),
			"parried":                  strconv.FormatBool(impact.Parried),
			"evaded":                   strconv.FormatBool(impact.Evaded),
			"pipeline_reason":          impact.Reason,
			"target_pipeline_state":    impact.TargetPipelineState,
			"target_iframe":            strconv.FormatBool(impact.TargetIFrame),
			"control_applied":          strconv.FormatBool(len(impact.StatusApplied) > 0),
			"status_applied":           strings.Join(impact.StatusApplied, ","),
			"control_type":             impact.ControlType,
			"control_release_policy":   impact.ControlReleasePolicy,
			"control_distance_cm":      formatImpactFloat(impact.ControlDistanceCM),
			"control_speed_cm_s":       formatImpactFloat(impact.ControlSpeedCMS),
			"control_direction_policy": impact.ControlDirectionPolicy,
			"damage_pipeline":          "combat_impact_resolution_v1",
			"feedback_authority":       "server_damage_event",
		}
		reason := "hit"
		switch {
		case impact.Evaded:
			reason = "evaded"
		case impact.Parried:
			reason = "parried"
		case impact.Blocked:
			reason = "blocked"
		case impact.DamageApplied <= 0 && impact.PostureApplied <= 0:
			reason = "negated"
		}
		events = append(events, &gamev1.SnapshotEvent{
			EventId:  fmt.Sprintf("damage:%d:%d:%s:%d", r.tick, impact.TargetID, impact.SkillID, index),
			Type:     gamev1.SnapshotEventType_ENTITY_EVENT_TYPE_DAMAGE_APPLIED,
			Reason:   reason,
			Tick:     r.tick,
			Source:   entityRef(source),
			Target:   entityRef(target),
			Metadata: metadata,
		})
	}
	return events
}

func impactProfileForEntity(entity *entityState) string {
	if entity == nil {
		return "default"
	}
	if profile := strings.TrimSpace(entity.impactResponseProfile); profile != "" {
		return profile
	}
	switch entity.entityType {
	case "player":
		return "flesh_blood_red"
	case "creature":
		return "creature_flesh_blood_red"
	default:
		return "default"
	}
}

func entityRef(entity *entityState) *gamev1.EntityRef {
	if entity == nil {
		return nil
	}
	return &gamev1.EntityRef{
		RuntimeEntityId: entity.id,
		EntityType:      entity.entityType,
		RegionId:        entity.regionID,
	}
}

func formatImpactFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 3, 64)
}
