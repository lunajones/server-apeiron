package ids

type RuntimeEntityID uint64
type RegionID string
type SkillID string
type SkillSetID string
type ItemTemplateID string
type CreatureTemplateID string
type PlayerID string

const InvalidRuntimeEntityID RuntimeEntityID = 0

func (id RuntimeEntityID) Valid() bool {
	return id != InvalidRuntimeEntityID
}
