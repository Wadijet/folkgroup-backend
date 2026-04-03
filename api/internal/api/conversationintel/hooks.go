// Package conversationintel — Hook sau datachanged fb_message_items: xếp job cix_intel_compute.
package conversationintel

import (
	"context"
	"strings"

	cixsvc "meta_commerce/internal/api/cix/service"
	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func stringField(m bson.M, k string) string {
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func resolveCustomerIDForConversation(ctx context.Context, conversationID string, ownerOrgID primitive.ObjectID) string {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" || ownerOrgID.IsZero() {
		return ""
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok || coll == nil {
		return ""
	}
	var doc struct {
		CustomerID string `bson:"customerId"`
	}
	err := coll.FindOne(ctx, bson.M{
		"conversationId":      conversationID,
		"ownerOrganizationId": ownerOrgID,
	}).Decode(&doc)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(doc.CustomerID)
}

// EnqueueCixIntelComputeFromDatachanged — Luồng 1: xếp thẳng job cix_intel_compute khi nguồn là fb_message_items.
func EnqueueCixIntelComputeFromDatachanged(ctx context.Context, e events.DataChangeEvent, normalizedRecordUid string) error {
	if e.Document == nil {
		return nil
	}
	ownerOrgID := events.GetOwnerOrganizationIDFromDocument(e.Document)
	if ownerOrgID.IsZero() {
		return nil
	}
	raw, ok := e.Document.(bson.M)
	if !ok {
		return nil
	}
	convID := strings.TrimSpace(stringField(raw, "conversationId"))
	if convID == "" {
		convID = strings.TrimSpace(stringField(raw, "ConversationId"))
	}
	if convID == "" {
		return nil
	}
	customerID := resolveCustomerIDForConversation(ctx, convID, ownerOrgID)
	svc, err := cixsvc.NewCixQueueService()
	if err != nil {
		return err
	}
	return svc.EnqueueAnalysis(ctx, cixsvc.EnqueueAnalysisInput{
		ConversationID:      convID,
		CustomerID:          customerID,
		Channel:             "messenger",
		CioEventUid:         strings.TrimSpace(normalizedRecordUid),
		OwnerOrganizationID: ownerOrgID,
	})
}
