package voucher

import "github.com/garyjia/ai-reimbursement/internal/models"

// AccountingSubjectMapper maps item types to Chinese accounting subjects
// ARCH-013-B: Pure function implementation for accounting subject mapping
type AccountingSubjectMapper struct{}

// NewAccountingSubjectMapper creates a new AccountingSubjectMapper
func NewAccountingSubjectMapper() *AccountingSubjectMapper {
	return &AccountingSubjectMapper{}
}

// MapToSubject returns the Chinese accounting subject (会计科目) for an item type
func (m *AccountingSubjectMapper) MapToSubject(itemType string) string {
	subjectMap := map[string]string{
		models.ItemTypeTravel:        "差旅费",
		models.ItemTypeMeal:          "餐费",
		models.ItemTypeAccommodation: "住宿费",
		models.ItemTypeEquipment:     "办公费",
		models.ItemTypeTransportation: "交通费",
		models.ItemTypeEntertainment: "业务招待费",
		models.ItemTypeTeamBuilding:  "福利费",
		models.ItemTypeCommunication: "通讯费",
		models.ItemTypeOther:         "其他费用",
	}

	if subject, ok := subjectMap[itemType]; ok {
		return subject
	}
	return "其他费用"
}

// MapToChineseName returns the Chinese display name for an item type
// This is used for the "报销类型" column in the form
func (m *AccountingSubjectMapper) MapToChineseName(itemType string) string {
	nameMap := map[string]string{
		models.ItemTypeTravel:        "差旅费",
		models.ItemTypeMeal:          "餐饮费",
		models.ItemTypeAccommodation: "住宿费",
		models.ItemTypeEquipment:     "办公用品",
		models.ItemTypeTransportation: "交通费",
		models.ItemTypeEntertainment: "招待费",
		models.ItemTypeTeamBuilding:  "团建费",
		models.ItemTypeCommunication: "通讯费",
		models.ItemTypeOther:         "其他",
	}

	if name, ok := nameMap[itemType]; ok {
		return name
	}
	return "其他"
}
