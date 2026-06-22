package mappers

import apeironv1 "db-apeiron/gen/apeiron/v1"

func ItemTemplateID(item *apeironv1.ItemTemplate) string {
	if item == nil {
		return ""
	}

	return item.Id
}
