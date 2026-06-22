package movement

import "sort"

type RuntimeActionContract struct {
	ID                       string
	AbilityKey               string
	ActionType               string
	DurationMS               int32
	AirborneDurationMS       int32
	ActiveMS                 int32
	RecoveryMS               int32
	DistanceCM               float64
	BaseSpeedCMS             float64
	SpeedCurveID             string
	SpeedCurveSamples        []MovementActionCurvePoint
	VerticalCurveSamples     []MovementActionCurvePoint
	JumpZVelocity            float64
	GravityScale             float64
	ExpectedApexMS           int32
	LandingDetectionPolicy   string
	GroundZPolicy            string
	CapsuleBaseOffset        float64
	AllowsAirControl         bool
	AirControlModifier       float64
	YawRateDegPerSec         float64
	ReconciliationContractID string
	ReconciliationCategory   string
	PhaseWindowPolicy        string
	PredictionErrorPolicy    string
	RootMotionOwner          string
	ContactPolicy            string
}

type ActionContractRegistry struct {
	contracts map[string]RuntimeActionContract
}

func NewActionContractRegistry(contracts map[string]RuntimeActionContract) ActionContractRegistry {
	copy := make(map[string]RuntimeActionContract, len(contracts))
	for key, contract := range contracts {
		copy[key] = contract
	}
	return ActionContractRegistry{contracts: copy}
}

func (r ActionContractRegistry) Resolve(abilityKey string) (RuntimeActionContract, bool) {
	if abilityKey == "" {
		return RuntimeActionContract{}, false
	}
	contract, ok := r.contracts[abilityKey]
	return contract, ok
}

func (r ActionContractRegistry) OrderedKeys(preferred []string) []string {
	seen := map[string]bool{}
	keys := make([]string, 0, len(r.contracts))
	for _, key := range preferred {
		if _, ok := r.contracts[key]; ok {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	var extra []string
	for key := range r.contracts {
		if !seen[key] {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)
	return append(keys, extra...)
}

func ReconciliationMode(contract RuntimeActionContract) string {
	if contract.ReconciliationCategory != "" {
		return contract.ReconciliationCategory
	}
	return contract.ReconciliationContractID
}

func ContractHash(contract RuntimeActionContract) string {
	if contract.ReconciliationContractID != "" {
		return contract.ReconciliationContractID
	}
	if contract.ReconciliationCategory != "" {
		return contract.ReconciliationCategory
	}
	return contract.ID
}

func ActionFamily(contract RuntimeActionContract) string {
	if contract.RootMotionOwner == "skill" || contract.ActionType == "grounded_skill" {
		return "skill_movement"
	}
	return "movement"
}
