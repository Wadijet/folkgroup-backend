// Package deliverydto - TrackingActionParams (xem dto.delivery.send.go cho package doc).
// File: dto.delivery.tracking.go - giữ tên cấu trúc cũ (dto.<domain>.<entity>.go).
package deliverydto

import (
	"fmt"
	"strconv"

	"meta_commerce/internal/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TrackingActionParams là params từ URL khi gọi tracking action
// Endpoint: GET /api/v1/track/:action/:historyId?ctaIndex=...
// Actions: "open", "click", "confirm", "cta"
type TrackingActionParams struct {
	Action    string             // Action: "open", "click", "confirm", "cta"
	HistoryID primitive.ObjectID // History ID từ URL params
	CTAIndex  int                // CTA index từ query (bắt buộc cho "click" và "cta")
}

// ParseHistoryID parse và validate historyId string sang ObjectID
func (p *TrackingActionParams) ParseHistoryID(historyIDStr string) (primitive.ObjectID, error) {
	if historyIDStr == "" {
		return primitive.NilObjectID, common.NewError(
			common.ErrCodeValidationFormat,
			"historyId is required",
			common.StatusBadRequest,
			nil,
		)
	}
	historyID, err := primitive.ObjectIDFromHex(historyIDStr)
	if err != nil {
		return primitive.NilObjectID, common.NewError(
			common.ErrCodeValidationFormat,
			fmt.Sprintf("Invalid historyId: %s", historyIDStr),
			common.StatusBadRequest,
			err,
		)
	}
	return historyID, nil
}

// ParseCTAIndex parse và validate ctaIndex string sang int (optional)
func (p *TrackingActionParams) ParseCTAIndex(ctaIndexStr string) (int, error) {
	if ctaIndexStr == "" {
		return -1, nil
	}
	ctaIndex, err := strconv.Atoi(ctaIndexStr)
	if err != nil {
		return -1, common.NewError(
			common.ErrCodeValidationFormat,
			fmt.Sprintf("Invalid ctaIndex: %s", ctaIndexStr),
			common.StatusBadRequest,
			err,
		)
	}
	return ctaIndex, nil
}
