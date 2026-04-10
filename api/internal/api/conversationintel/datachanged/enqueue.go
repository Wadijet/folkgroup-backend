// Package datachanged — Miền CIX: xếp job phân tích hội thoại từ datachanged fb_message_items.
package datachanged

import (
	"context"

	convintel "meta_commerce/internal/api/conversationintel"
	"meta_commerce/internal/api/events"
)

// EnqueueCixComputeFromDataChange giao việc cix_intel_compute cho domain conversation intelligence.
func EnqueueCixComputeFromDataChange(ctx context.Context, e events.DataChangeEvent, messageItemIDHex, traceID, correlationID string) error {
	return convintel.EnqueueCixIntelComputeFromDatachanged(ctx, e, messageItemIDHex, traceID, correlationID)
}
