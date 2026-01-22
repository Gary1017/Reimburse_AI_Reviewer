package voucher

import (
	"testing"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/stretchr/testify/assert"
)

// ARCH-013-B: AccountingSubjectMapper tests
// Tests for mapping item types to Chinese accounting subjects

func TestAccountingSubjectMapper_MapToSubject(t *testing.T) {
	mapper := NewAccountingSubjectMapper()

	tests := []struct {
		name     string
		itemType string
		expected string
	}{
		{
			name:     "maps TRAVEL to 差旅费",
			itemType: models.ItemTypeTravel,
			expected: "差旅费",
		},
		{
			name:     "maps MEAL to 餐费",
			itemType: models.ItemTypeMeal,
			expected: "餐费",
		},
		{
			name:     "maps ACCOMMODATION to 住宿费",
			itemType: models.ItemTypeAccommodation,
			expected: "住宿费",
		},
		{
			name:     "maps EQUIPMENT to 办公费",
			itemType: models.ItemTypeEquipment,
			expected: "办公费",
		},
		{
			name:     "maps TRANSPORTATION to 交通费",
			itemType: models.ItemTypeTransportation,
			expected: "交通费",
		},
		{
			name:     "maps ENTERTAINMENT to 业务招待费",
			itemType: models.ItemTypeEntertainment,
			expected: "业务招待费",
		},
		{
			name:     "maps TEAM_BUILDING to 福利费",
			itemType: models.ItemTypeTeamBuilding,
			expected: "福利费",
		},
		{
			name:     "maps COMMUNICATION to 通讯费",
			itemType: models.ItemTypeCommunication,
			expected: "通讯费",
		},
		{
			name:     "maps OTHER to 其他费用",
			itemType: models.ItemTypeOther,
			expected: "其他费用",
		},
		{
			name:     "maps unknown type to 其他费用",
			itemType: "UNKNOWN_TYPE",
			expected: "其他费用",
		},
		{
			name:     "maps empty string to 其他费用",
			itemType: "",
			expected: "其他费用",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.MapToSubject(tt.itemType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAccountingSubjectMapper_MapToChineseName(t *testing.T) {
	mapper := NewAccountingSubjectMapper()

	tests := []struct {
		name     string
		itemType string
		expected string
	}{
		{
			name:     "maps TRAVEL to 差旅费",
			itemType: models.ItemTypeTravel,
			expected: "差旅费",
		},
		{
			name:     "maps MEAL to 餐饮费",
			itemType: models.ItemTypeMeal,
			expected: "餐饮费",
		},
		{
			name:     "maps ACCOMMODATION to 住宿费",
			itemType: models.ItemTypeAccommodation,
			expected: "住宿费",
		},
		{
			name:     "maps EQUIPMENT to 办公用品",
			itemType: models.ItemTypeEquipment,
			expected: "办公用品",
		},
		{
			name:     "maps TRANSPORTATION to 交通费",
			itemType: models.ItemTypeTransportation,
			expected: "交通费",
		},
		{
			name:     "maps ENTERTAINMENT to 招待费",
			itemType: models.ItemTypeEntertainment,
			expected: "招待费",
		},
		{
			name:     "maps TEAM_BUILDING to 团建费",
			itemType: models.ItemTypeTeamBuilding,
			expected: "团建费",
		},
		{
			name:     "maps COMMUNICATION to 通讯费",
			itemType: models.ItemTypeCommunication,
			expected: "通讯费",
		},
		{
			name:     "maps OTHER to 其他",
			itemType: models.ItemTypeOther,
			expected: "其他",
		},
		{
			name:     "maps unknown type to 其他",
			itemType: "UNKNOWN_TYPE",
			expected: "其他",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.MapToChineseName(tt.itemType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewAccountingSubjectMapper(t *testing.T) {
	mapper := NewAccountingSubjectMapper()
	assert.NotNil(t, mapper)
}
