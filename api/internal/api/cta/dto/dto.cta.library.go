// Package ctadto chứa DTO cho domain CTA (Library).
// File: dto.cta.library.go - giữ tên cấu trúc cũ (dto.<domain>.<entity>.go).
package ctadto

import (
	"fmt"
	"strconv"

	"meta_commerce/internal/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CTALibraryCreateInput là input để tạo CTA Library
type CTALibraryCreateInput struct {
	Code        string   `json:"code" validate:"required"`        // Mã CTA (unique trong organization)
	Label       string   `json:"label" validate:"required"`        // Label hiển thị
	Action      string   `json:"action" validate:"required"`       // URL action
	Style       string   `json:"style,omitempty"`                 // Style: "primary", "success", "secondary", "danger"
	Variables   []string `json:"variables"`                       // Danh sách variables
	Description string   `json:"description,omitempty"`          // Mô tả về CTA để người dùng hiểu được mục đích sử dụng
	IsActive    bool     `json:"isActive"`                        // Trạng thái hoạt động
}

// CTALibraryUpdateInput là input để cập nhật CTA Library
type CTALibraryUpdateInput struct {
	Label       *string  `json:"label,omitempty"`       // Label hiển thị
	Action      *string  `json:"action,omitempty"`      // URL action
	Style       *string  `json:"style,omitempty"`       // Style
	Variables   *[]string `json:"variables,omitempty"`  // Danh sách variables
	Description *string  `json:"description,omitempty"` // Mô tả về CTA để người dùng hiểu được mục đích sử dụng
	IsActive    *bool    `json:"isActive,omitempty"`   // Trạng thái hoạt động
}

// CTAActionParams là params từ URL khi gọi CTA action (DEPRECATED - đã gộp vào tracking)
// Giữ để backward compatibility.
type CTAActionParams struct {
	Action    string             // Action: "track", "render", "preview", etc.
	HistoryID primitive.ObjectID // History ID từ URL params
	CTAIndex  int                // CTA index từ URL params
}

// ParseHistoryID parse và validate historyId string sang ObjectID
func (p *CTAActionParams) ParseHistoryID(historyIDStr string) (primitive.ObjectID, error) {
	if historyIDStr == "" {
		return primitive.NilObjectID, common.NewError(common.ErrCodeValidationFormat, "historyId is required", common.StatusBadRequest, nil)
	}
	historyID, err := primitive.ObjectIDFromHex(historyIDStr)
	if err != nil {
		return primitive.NilObjectID, common.NewError(common.ErrCodeValidationFormat, fmt.Sprintf("Invalid historyId: %s", historyIDStr), common.StatusBadRequest, err)
	}
	return historyID, nil
}

// ParseCTAIndex parse và validate ctaIndex string sang int
func (p *CTAActionParams) ParseCTAIndex(ctaIndexStr string) (int, error) {
	if ctaIndexStr == "" {
		return 0, common.NewError(common.ErrCodeValidationFormat, "ctaIndex is required", common.StatusBadRequest, nil)
	}
	ctaIndex, err := strconv.Atoi(ctaIndexStr)
	if err != nil {
		return 0, common.NewError(common.ErrCodeValidationFormat, fmt.Sprintf("Invalid ctaIndex: %s", ctaIndexStr), common.StatusBadRequest, err)
	}
	return ctaIndex, nil
}
