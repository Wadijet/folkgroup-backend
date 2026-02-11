package utility

import (
	"fmt"
	contentmodels "meta_commerce/internal/api/content/models"
	"meta_commerce/internal/common"
)

// ContentLevelMap ánh xạ content type sang level number (L1-L6)
var ContentLevelMap = map[string]int{
	contentmodels.ContentNodeTypePillar:      1, // L1
	contentmodels.ContentNodeTypeSTP:         2, // L2
	contentmodels.ContentNodeTypeInsight:     3, // L3
	contentmodels.ContentNodeTypeContentLine: 4, // L4
	contentmodels.ContentNodeTypeGene:        5, // L5
	contentmodels.ContentNodeTypeScript:      6, // L6
}

// GetContentLevel trả về level number (1-6) của content type
// Trả về 0 nếu type không hợp lệ
func GetContentLevel(contentType string) int {
	if level, exists := ContentLevelMap[contentType]; exists {
		return level
	}
	return 0
}

// GetExpectedParentLevel trả về level number của parent node mong đợi cho một content type
// Ví dụ: STP (L2) cần parent là Pillar (L1) → trả về 1
// Trả về 0 nếu là root level (Pillar - L1) hoặc type không hợp lệ
func GetExpectedParentLevel(contentType string) int {
	currentLevel := GetContentLevel(contentType)
	if currentLevel <= 1 {
		return 0 // Root level hoặc không hợp lệ
	}
	return currentLevel - 1
}

// GetExpectedParentType trả về content type của parent node mong đợi
// Ví dụ: STP (L2) cần parent là Pillar (L1) → trả về "pillar"
// Trả về "" nếu là root level (Pillar - L1) hoặc type không hợp lệ
func GetExpectedParentType(contentType string) string {
	expectedLevel := GetExpectedParentLevel(contentType)
	if expectedLevel == 0 {
		return ""
	}

	// Tìm type tương ứng với expected level
	for t, level := range ContentLevelMap {
		if level == expectedLevel {
			return t
		}
	}
	return ""
}

// ValidateSequentialLevelConstraint kiểm tra ràng buộc tuần tự level:
// 1. Content type phải hợp lệ (có trong ContentLevelMap)
// 2. Nếu có parent, parent phải tồn tại và có level đúng (level = currentLevel - 1)
// 3. Parent phải đã được commit (production) hoặc là draft đã được approve (nếu là draft)
//
// Tham số:
//   - contentType: Type của content node cần validate
//   - parentType: Type của parent node (có thể là "" nếu không có parent)
//   - parentExists: true nếu parent tồn tại trong database
//   - parentIsProduction: true nếu parent đã được commit (production), false nếu là draft
//   - parentIsApproved: true nếu parent là draft đã được approve (chỉ áp dụng khi parentIsProduction = false)
//
// Trả về:
//   - error: Lỗi nếu vi phạm ràng buộc, nil nếu hợp lệ
func ValidateSequentialLevelConstraint(
	contentType string,
	parentType string,
	parentExists bool,
	parentIsProduction bool,
	parentIsApproved bool,
) error {
	// 1. Kiểm tra content type hợp lệ
	currentLevel := GetContentLevel(contentType)
	if currentLevel == 0 {
		return common.NewError(
			common.ErrCodeValidationFormat,
			fmt.Sprintf("Content type '%s' không hợp lệ. Các type hợp lệ: pillar, stp, insight, contentLine, gene, script", contentType),
			common.StatusBadRequest,
			nil,
		)
	}

	// 2. Nếu là root level (L1 - Pillar), không cần parent
	if currentLevel == 1 {
		if parentType != "" {
			return common.NewError(
				common.ErrCodeBusinessOperation,
				"Pillar (L1) là root level, không được có parent",
				common.StatusBadRequest,
				nil,
			)
		}
		return nil // Root level hợp lệ
	}

	// 3. Nếu không phải root level, PHẢI có parent
	if parentType == "" {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("Content type '%s' (L%d) phải có parent node. Parent mong đợi: %s (L%d)",
				contentType, currentLevel,
				GetExpectedParentType(contentType), GetExpectedParentLevel(contentType)),
			common.StatusBadRequest,
			nil,
		)
	}

	// 4. Kiểm tra parent type đúng level
	parentLevel := GetContentLevel(parentType)
	expectedParentLevel := GetExpectedParentLevel(contentType)
	if parentLevel != expectedParentLevel {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("Parent type '%s' (L%d) không đúng level. Content type '%s' (L%d) cần parent là %s (L%d)",
				parentType, parentLevel,
				contentType, currentLevel,
				GetExpectedParentType(contentType), expectedParentLevel),
			common.StatusBadRequest,
			nil,
		)
	}

	// 5. Kiểm tra parent phải tồn tại
	if !parentExists {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			fmt.Sprintf("Parent node (type: %s, L%d) không tồn tại. Phải tạo parent trước khi tạo %s (L%d)",
				parentType, parentLevel,
				contentType, currentLevel),
			common.StatusBadRequest,
			nil,
		)
	}

	// 6. Kiểm tra parent phải đã được commit (production) hoặc là draft đã được approve
	if !parentIsProduction {
		// Parent là draft, phải đã được approve
		if !parentIsApproved {
			return common.NewError(
				common.ErrCodeBusinessOperation,
				fmt.Sprintf("Parent node (type: %s, L%d) là draft chưa được approve. Phải approve và commit parent trước khi tạo %s (L%d)",
					parentType, parentLevel,
					contentType, currentLevel),
				common.StatusBadRequest,
				nil,
			)
		}
	}

	// Tất cả validation đều pass
	return nil
}
