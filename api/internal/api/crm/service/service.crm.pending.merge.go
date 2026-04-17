// Package crmvc — Queue crm_pending_merge: xếp job merge L1→L2 (tách tên khỏi CIO ingest).
package crmvc

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"

	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	basesvc "meta_commerce/internal/api/base/service"
	crmmodels "meta_commerce/internal/api/crm/models"
	"meta_commerce/internal/api/events"
	fbmodels "meta_commerce/internal/api/fb/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// CrmPendingMergeService CRUD cho crm_pending_merge.
type CrmPendingMergeService struct {
	*basesvc.BaseServiceMongoImpl[crmmodels.CrmPendingMerge]
}

// NewCrmPendingMergeService tạo service CRUD cho crm_pending_merge.
func NewCrmPendingMergeService() (*CrmPendingMergeService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CustomerPendingMerge)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.CustomerPendingMerge, common.ErrNotFound)
	}
	return &CrmPendingMergeService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[crmmodels.CrmPendingMerge](coll),
	}, nil
}

// applyDomainQueueBusToPendingMergeSet ghi eventType/eventSource/pipelineStage; giữ giá trị cũ nếu bus rỗng từng field và có prev.
func applyDomainQueueBusToPendingMergeSet(set bson.M, bus *crmqueue.DomainQueueBusFields, prev *crmmodels.CrmPendingMerge) {
	if bus == nil {
		return
	}
	mergeOne := func(key, fromBus, prevVal string) {
		v := strings.TrimSpace(fromBus)
		if v != "" {
			set[key] = v
			return
		}
		if prev != nil {
			if p := strings.TrimSpace(prevVal); p != "" {
				set[key] = p
			}
		}
	}
	mergeOne("eventType", bus.EventType, prev.EventType)
	mergeOne("eventSource", bus.EventSource, prev.EventSource)
	mergeOne("pipelineStage", bus.PipelineStage, prev.PipelineStage)
	mergeOne("ownerDomain", bus.OwnerDomain, prev.OwnerDomain)
	mergeOne("processorDomain", bus.ProcessorDomain, prev.ProcessorDomain)
	mergeOne("enqueueSourceDomain", bus.EnqueueSourceDomain, prev.EnqueueSourceDomain)
	mergeOne("e2eStage", bus.E2EStage, prev.E2EStage)
	mergeOne("e2eStepId", bus.E2EStepID, prev.E2EStepID)
}

// EnqueueCrmPendingMerge thêm hoặc cập nhật job trong crm_pending_merge.
// Coalesce + debounce: env CRM_MERGE_QUEUE_COALESCE_* (fallback CRM_INGEST_COALESCE_*).
// traceID / correlationID từ consumer AID (datachanged); rỗng nếu flush defer — không xóa trace đã có khi cập nhật coalesce.
// bus — bản sao envelope bus AID (có thể nil).
func EnqueueCrmPendingMerge(ctx context.Context, collectionName, operation string, document interface{}, prevDoc interface{}, ownerOrgID primitive.ObjectID, traceID, correlationID string, bus *crmqueue.DomainQueueBusFields) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CustomerPendingMerge)
	if !ok {
		return fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.CustomerPendingMerge)
	}
	docMap, err := documentToBsonM(document)
	if err != nil {
		return err
	}
	businessKey, ok := extractBusinessKey(collectionName, docMap, ownerOrgID)
	if !ok {
		return fmt.Errorf("không thể trích businessKey từ document")
	}
	now := time.Now().Unix()

	updatedAtNew := events.ExtractUpdatedAtFromDoc(collectionName, document)
	updatedAtOld := int64(0)
	if prevDoc != nil {
		updatedAtOld = events.ExtractUpdatedAtFromDoc(collectionName, prevDoc)
	}
	updatedAtDeltaMs := int64(-1)
	if updatedAtNew > 0 && updatedAtOld > 0 {
		updatedAtDeltaMs = updatedAtNew - updatedAtOld
	}

	if crmMergeQueueCoalesceEnabled() {
		inboxCID := extractInboxCustomerIDFromDocMap(collectionName, docMap)
		if inboxCID != "" {
			coalesceKey := buildCoalesceKey(ownerOrgID, inboxCID)
			debounceSec := crmMergeQueueCoalesceDebounceSec()
			return upsertCoalescedCrmPendingMerge(ctx, coll, collectionName, operation, docMap, ownerOrgID, coalesceKey, inboxCID, debounceSec, now, updatedAtNew, updatedAtOld, updatedAtDeltaMs, traceID, correlationID, bus)
		}
	}

	setNonCoalesced := bson.M{
		"collectionName":      collectionName,
		"operation":           operation,
		"document":            docMap,
		"ownerOrganizationId": ownerOrgID,
		"createdAt":           now,
		"processedAt":         nil,
		"processError":        "",
		"updatedAtNew":        updatedAtNew,
		"updatedAtOld":        updatedAtOld,
		"updatedAtDeltaMs":    updatedAtDeltaMs,
		"sourceCollections":   []string{collectionName},
		"sourceSnapshots":     []crmmodels.CrmPendingMergeSnapshot{},
		"coalesceKey":         "",
		"inboxCustomerId":     "",
		"mergeNotBefore":      int64(0),
	}
	if t := strings.TrimSpace(traceID); t != "" {
		setNonCoalesced["traceId"] = t
	}
	if c := strings.TrimSpace(correlationID); c != "" {
		setNonCoalesced["correlationId"] = c
	}
	applyDomainQueueBusToPendingMergeSet(setNonCoalesced, bus, nil)

	filter := bson.M{"businessKey": businessKey}
	update := bson.M{"$set": setNonCoalesced}
	opts := mongoopts.Update().SetUpsert(true)
	_, err = coll.UpdateOne(ctx, filter, update, opts)
	return err
}

func extractEntityPartFromDocMap(collectionName string, docMap bson.M) (part string, ok bool) {
	switch collectionName {
	case global.MongoDB_ColNames.PcPosCustomers, global.MongoDB_ColNames.FbCustomers:
		part, _ = getStringFromMap(docMap, "customerId")
	case global.MongoDB_ColNames.PcPosOrders:
		if v, ok := docMap["orderId"]; ok {
			part = fmt.Sprintf("%v", v)
		}
	case global.MongoDB_ColNames.FbConvesations:
		part, _ = getStringFromMap(docMap, "conversationId")
	case global.MongoDB_ColNames.CustomerNotes:
		if v, ok := docMap["_id"]; ok {
			if oid, ok := v.(primitive.ObjectID); ok {
				part = oid.Hex()
			}
		}
	}
	part = strings.TrimSpace(part)
	return part, part != ""
}

func extractBusinessKey(collectionName string, docMap bson.M, ownerOrgID primitive.ObjectID) (string, bool) {
	orgHex := ownerOrgID.Hex()
	if orgHex == "" || orgHex == "000000000000000000000000" {
		return "", false
	}
	part, ok := extractEntityPartFromDocMap(collectionName, docMap)
	if !ok {
		return "", false
	}
	return collectionName + "|" + orgHex + "|" + part, true
}

func crmMergeQueueCoalesceEnabled() bool {
	s := strings.TrimSpace(strings.ToLower(os.Getenv("CRM_MERGE_QUEUE_COALESCE_ENABLED")))
	if s != "" {
		return s == "1" || s == "true" || s == "yes"
	}
	legacy := strings.TrimSpace(strings.ToLower(os.Getenv("CRM_INGEST_COALESCE_ENABLED")))
	return legacy == "" || legacy == "1" || legacy == "true" || legacy == "yes"
}

func crmMergeQueueCoalesceDebounceSec() int {
	s := strings.TrimSpace(os.Getenv("CRM_MERGE_QUEUE_COALESCE_DEBOUNCE_SEC"))
	if s != "" {
		n, err := strconv.Atoi(s)
		if err == nil && n >= 0 {
			return n
		}
		return 3
	}
	s2 := strings.TrimSpace(os.Getenv("CRM_INGEST_COALESCE_DEBOUNCE_SEC"))
	if s2 != "" {
		n, err := strconv.Atoi(s2)
		if err == nil && n >= 0 {
			return n
		}
	}
	return 3
}

func buildCoalesceKey(ownerOrgID primitive.ObjectID, inboxCustomerID string) string {
	orgHex := ownerOrgID.Hex()
	inboxCustomerID = strings.TrimSpace(inboxCustomerID)
	if orgHex == "" || inboxCustomerID == "" {
		return ""
	}
	return "crm_coalesce|" + orgHex + "|" + inboxCustomerID
}

func extractInboxCustomerIDFromDocMap(collectionName string, docMap bson.M) string {
	if docMap == nil {
		return ""
	}
	cn := strings.TrimSpace(collectionName)
	switch cn {
	case global.MongoDB_ColNames.PcPosCustomers:
		s, _ := getStringFromMap(docMap, "customerId")
		return strings.TrimSpace(s)
	case global.MongoDB_ColNames.FbCustomers:
		s, _ := getStringFromMap(docMap, "customerId")
		return strings.TrimSpace(s)
	case global.MongoDB_ColNames.PcPosOrders:
		var d pcmodels.PcPosOrder
		if err := bsonMapToStructPending(docMap, &d); err != nil {
			return ""
		}
		cid := strings.TrimSpace(d.CustomerId)
		if cid == "" && d.PosData != nil {
			if m, ok := d.PosData["customer"].(map[string]interface{}); ok {
				if id, ok := m["id"].(string); ok {
					cid = strings.TrimSpace(id)
				}
			}
		}
		return cid
	case global.MongoDB_ColNames.FbConvesations:
		var d fbmodels.FbConversation
		if err := bsonMapToStructPending(docMap, &d); err != nil {
			return ""
		}
		return strings.TrimSpace(ExtractConversationCustomerId(&d))
	case global.MongoDB_ColNames.CustomerNotes:
		var d crmmodels.CrmNote
		if err := bsonMapToStructPending(docMap, &d); err != nil {
			return ""
		}
		return strings.TrimSpace(d.CustomerId)
	default:
		return ""
	}
}

func bsonMapToStructPending(m bson.M, out interface{}) error {
	if m == nil {
		return nil
	}
	data, err := bson.Marshal(m)
	if err != nil {
		return err
	}
	return bson.Unmarshal(data, out)
}

func upsertCoalescedCrmPendingMerge(ctx context.Context, coll *mongo.Collection, collectionName, operation string, docMap bson.M, ownerOrgID primitive.ObjectID, coalesceKey, inboxCustomerID string, debounceSec int, now, updatedAtNew, updatedAtOld, updatedAtDeltaMs int64, traceID, correlationID string, bus *crmqueue.DomainQueueBusFields) error {
	entityPart, ok := extractEntityPartFromDocMap(collectionName, docMap)
	if !ok {
		return fmt.Errorf("không trích entity part cho snapshot coalesce")
	}
	snapshotKey := collectionName + "|" + entityPart
	snap := crmmodels.CrmPendingMergeSnapshot{
		CollectionName: collectionName,
		SnapshotKey:    snapshotKey,
		Operation:      operation,
		Document:       cloneBsonMDoc(docMap),
	}

	var mergeNotBefore int64
	if debounceSec > 0 {
		mergeNotBefore = now + int64(debounceSec)
	}

	filter := bson.M{"coalesceKey": coalesceKey, "processedAt": nil}
	var cur crmmodels.CrmPendingMerge
	err := coll.FindOne(ctx, filter).Decode(&cur)
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	if err == mongo.ErrNoDocuments {
		job := crmmodels.CrmPendingMerge{
			ID:                  primitive.NewObjectID(),
			CoalesceKey:         coalesceKey,
			BusinessKey:         coalesceKey,
			CollectionName:      collectionName,
			Operation:           operation,
			Document:            cloneBsonMDoc(docMap),
			OwnerOrganizationID: ownerOrgID,
			CreatedAt:           now,
			ProcessedAt:         nil,
			ProcessError:        "",
			UpdatedAtNew:        updatedAtNew,
			UpdatedAtOld:        updatedAtOld,
			UpdatedAtDeltaMs:    updatedAtDeltaMs,
			SourceCollections:   []string{collectionName},
			SourceSnapshots:     []crmmodels.CrmPendingMergeSnapshot{snap},
			InboxCustomerId:     inboxCustomerID,
			MergeNotBefore:      mergeNotBefore,
			TraceID:             strings.TrimSpace(traceID),
			CorrelationID:       strings.TrimSpace(correlationID),
		}
		if bus != nil {
			job.EventType = strings.TrimSpace(bus.EventType)
			job.EventSource = strings.TrimSpace(bus.EventSource)
			job.PipelineStage = strings.TrimSpace(bus.PipelineStage)
			job.OwnerDomain = strings.TrimSpace(bus.OwnerDomain)
			job.ProcessorDomain = strings.TrimSpace(bus.ProcessorDomain)
			job.EnqueueSourceDomain = strings.TrimSpace(bus.EnqueueSourceDomain)
			job.E2EStage = strings.TrimSpace(bus.E2EStage)
			job.E2EStepID = strings.TrimSpace(bus.E2EStepID)
		}
		_, insErr := coll.InsertOne(ctx, job)
		return insErr
	}

	mergedSnaps := mergeCrmPendingMergeSnapshots(cur.SourceSnapshots, snap)
	srcCols := unionStringSlicePreserveOrder(cur.SourceCollections, collectionName)

	mb := mergeNotBefore
	if debounceSec > 0 {
		mb = now + int64(debounceSec)
	}

	tid := strings.TrimSpace(traceID)
	if tid == "" {
		tid = strings.TrimSpace(cur.TraceID)
	}
	cid := strings.TrimSpace(correlationID)
	if cid == "" {
		cid = strings.TrimSpace(cur.CorrelationID)
	}

	setCoalesced := bson.M{
		"collectionName":      collectionName,
		"operation":           operation,
		"document":            cloneBsonMDoc(docMap),
		"ownerOrganizationId": ownerOrgID,
		"createdAt":           now,
		"processedAt":         nil,
		"processError":        "",
		"updatedAtNew":        updatedAtNew,
		"updatedAtOld":        updatedAtOld,
		"updatedAtDeltaMs":    updatedAtDeltaMs,
		"sourceSnapshots":     mergedSnaps,
		"sourceCollections":   srcCols,
		"inboxCustomerId":     inboxCustomerID,
		"mergeNotBefore":      mb,
		"businessKey":         coalesceKey,
	}
	if tid != "" {
		setCoalesced["traceId"] = tid
	}
	if cid != "" {
		setCoalesced["correlationId"] = cid
	}
	applyDomainQueueBusToPendingMergeSet(setCoalesced, bus, &cur)

	_, err = coll.UpdateOne(ctx, bson.M{"_id": cur.ID}, bson.M{"$set": setCoalesced})
	return err
}

func cloneBsonMDoc(m bson.M) bson.M {
	if m == nil {
		return nil
	}
	out, err := documentToBsonM(m)
	if err != nil {
		return m
	}
	return out
}

func mergeCrmPendingMergeSnapshots(existing []crmmodels.CrmPendingMergeSnapshot, incoming crmmodels.CrmPendingMergeSnapshot) []crmmodels.CrmPendingMergeSnapshot {
	byKey := make(map[string]crmmodels.CrmPendingMergeSnapshot)
	order := make([]string, 0, len(existing)+1)
	for _, s := range existing {
		k := strings.TrimSpace(s.SnapshotKey)
		if k == "" {
			continue
		}
		byKey[k] = s
		order = append(order, k)
	}
	ink := strings.TrimSpace(incoming.SnapshotKey)
	if ink != "" {
		if _, ok := byKey[ink]; !ok {
			order = append(order, ink)
		}
		byKey[ink] = incoming
	}
	out := make([]crmmodels.CrmPendingMergeSnapshot, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out
}

func unionStringSlicePreserveOrder(a []string, add string) []string {
	add = strings.TrimSpace(add)
	if add == "" {
		return a
	}
	seen := make(map[string]struct{})
	for _, x := range a {
		seen[strings.TrimSpace(x)] = struct{}{}
	}
	out := append([]string(nil), a...)
	if _, ok := seen[add]; !ok {
		out = append(out, add)
	}
	return out
}

func crmPendingMergeDueFilter() bson.M {
	now := time.Now().Unix()
	return bson.M{
		"processedAt": nil,
		"$or": []bson.M{
			{"mergeNotBefore": bson.M{"$exists": false}},
			{"mergeNotBefore": bson.M{"$lte": 0}},
			{"mergeNotBefore": bson.M{"$lte": now}},
		},
	}
}

func documentToBsonM(doc interface{}) (bson.M, error) {
	if doc == nil {
		return bson.M{}, nil
	}
	data, err := bson.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var m bson.M
	if err := bson.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// CountUnprocessedCrmPendingMerge đếm job chưa xử lý (đã đến hạn debounce).
func CountUnprocessedCrmPendingMerge(ctx context.Context) (int64, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CustomerPendingMerge)
	if !ok {
		return 0, fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.CustomerPendingMerge)
	}
	return coll.CountDocuments(ctx, crmPendingMergeDueFilter())
}

// GetUnprocessedCrmPendingMerge lấy tối đa limit job đến hạn.
func GetUnprocessedCrmPendingMerge(ctx context.Context, limit int) ([]crmmodels.CrmPendingMerge, error) {
	if limit <= 0 {
		limit = 50
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CustomerPendingMerge)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.CustomerPendingMerge)
	}
	filter := crmPendingMergeDueFilter()
	opts := mongoopts.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit))
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var list []crmmodels.CrmPendingMerge
	if err := cursor.All(ctx, &list); err != nil {
		return nil, err
	}
	if list == nil {
		list = []crmmodels.CrmPendingMerge{}
	}
	return list, nil
}

// SetCrmPendingMergeProcessed đánh dấu job đã xử lý.
func SetCrmPendingMergeProcessed(ctx context.Context, id primitive.ObjectID, processErr string) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CustomerPendingMerge)
	if !ok {
		return fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.CustomerPendingMerge)
	}
	now := time.Now().Unix()
	update := bson.M{"$set": bson.M{"processedAt": now, "processError": processErr}}
	_, err := coll.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}
