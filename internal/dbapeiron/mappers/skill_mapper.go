package mappers

import apeironv1 "db-apeiron/gen/apeiron/v1"

func SkillID(skill *apeironv1.Skill) string {
	if skill == nil {
		return ""
	}

	return skill.Id
}
