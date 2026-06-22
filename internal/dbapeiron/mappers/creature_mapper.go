package mappers

import apeironv1 "db-apeiron/gen/apeiron/v1"

func CreatureTemplateID(template *apeironv1.CreatureTemplate) string {
	if template == nil {
		return ""
	}

	return template.Id
}
