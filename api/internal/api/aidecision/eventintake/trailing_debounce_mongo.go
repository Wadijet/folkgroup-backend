// Package eventintake — Trailing debounce ghi MongoDB (decision_trailing_debounce), thay cho queuedebounce in-process.
//
// Upsert dùng UpdateOne với aggregation pipeline (cần MongoDB 4.2+).
package eventintake

import (
	"context"
	"fmt"
	"strings"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	trailingBucketDatachangedDefer    = "datachanged_defer"
	trailingBucketCrmIntelAfterIngest = "crm_intel_after_ingest"
)

func trailingDebounceColl() *mongo.Collection {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionTrailingDebounce)
	if !ok || coll == nil {
		return nil
	}
	return coll
}

func datachangedDeferDebounceKey(kind DeferredSideEffectKind, orgHex, collName, idHex string) string {
	return fmt.Sprintf("dd:%s:%s:%s:%s", kind, orgHex, collName, idHex)
}

func crmIntelAfterIngestDebounceKey(orgHex, unifiedID string) string {
	return fmt.Sprintf("crm_ingest:%s:%s", orgHex, unifiedID)
}

func mergeTraceExpr(field string, literal string) bson.M {
	lit := strings.TrimSpace(literal)
	return bson.M{
		"$cond": bson.A{
			bson.M{"$ne": bson.A{lit, ""}},
			lit,
			bson.M{"$ifNull": bson.A{"$" + field, ""}},
		},
	}
}

// upsertTrailingDatachangedDefer — trailing: mỗi lần gọi lùi dueAtMs; trace/correlation gộp giữ giá trị mới nếu khác rỗng.
func upsertTrailingDatachangedDefer(ctx context.Context, kind DeferredSideEffectKind, orgHex, collName, idHex string, window time.Duration, traceID, correlationID string) error {
	coll := trailingDebounceColl()
	if coll == nil || window <= 0 {
		return nil
	}
	orgHex = strings.TrimSpace(orgHex)
	collName = strings.TrimSpace(collName)
	idHex = strings.TrimSpace(idHex)
	if orgHex == "" || collName == "" || idHex == "" {
		return nil
	}
	key := datachangedDeferDebounceKey(kind, orgHex, collName, idHex)
	nowMs := time.Now().UnixMilli()
	dueAt := nowMs + window.Milliseconds()
	ownerOID, _ := primitive.ObjectIDFromHex(orgHex)

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.M{
			"debounceKey":   key,
			"bucket":        trailingBucketDatachangedDefer,
			"dueAtMs":       dueAt,
			"deferKind":     string(kind),
			"orgHex":        orgHex,
			"sourceColl":    collName,
			"idHex":         idHex,
			"ownerOrgId":    ownerOID,
			"updatedAtMs":   nowMs,
			"createdAtMs":   bson.M{"$ifNull": bson.A{"$createdAtMs", nowMs}},
			"traceId":       mergeTraceExpr("traceId", traceID),
			"correlationId": mergeTraceExpr("correlationId", correlationID),
		}}},
	}
	_, err := coll.UpdateOne(ctx, bson.M{"debounceKey": key}, pipeline, options.Update().SetUpsert(true))
	return err
}

// upsertTrailingCrmIntelAfterIngest — trailing + gộp causalMs theo mergeCausalMsDebounced.
func upsertTrailingCrmIntelAfterIngest(ctx context.Context, orgHex, unifiedID string, window time.Duration, traceID, correlationID, parentEventID string, causalOrderingAtMs int64) error {
	coll := trailingDebounceColl()
	if coll == nil {
		return nil
	}
	if window <= 0 {
		window = CrmIntelAfterIngestDebounceWindow()
	}
	if window <= 0 {
		return nil
	}
	orgHex = strings.TrimSpace(orgHex)
	unifiedID = strings.TrimSpace(unifiedID)
	if orgHex == "" || unifiedID == "" {
		return nil
	}
	key := crmIntelAfterIngestDebounceKey(orgHex, unifiedID)
	nowMs := time.Now().UnixMilli()
	dueAt := nowMs + window.Milliseconds()
	ownerOID, _ := primitive.ObjectIDFromHex(orgHex)

	// causalMs: $let p = ifNull(existing,0), n = literal; logic khớp mergeCausalMsDebounced
	causalExpr := bson.M{
		"$let": bson.M{
			"vars": bson.M{
				"p": bson.M{"$ifNull": bson.A{"$causalMs", int64(0)}},
				"n": causalOrderingAtMs,
			},
			"in": bson.M{
				"$cond": bson.A{
					bson.M{"$lte": bson.A{"$$p", int64(0)}},
					"$$n",
					bson.M{
						"$cond": bson.A{
							bson.M{"$lte": bson.A{"$$n", int64(0)}},
							"$$p",
							bson.M{"$max": bson.A{"$$p", "$$n"}},
						},
					},
				},
			},
		},
	}

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.M{
			"debounceKey":   key,
			"bucket":        trailingBucketCrmIntelAfterIngest,
			"dueAtMs":       dueAt,
			"orgHex":        orgHex,
			"ownerOrgId":    ownerOID,
			"unifiedId":     unifiedID,
			"updatedAtMs":   nowMs,
			"createdAtMs":   bson.M{"$ifNull": bson.A{"$createdAtMs", nowMs}},
			"traceId":       mergeTraceExpr("traceId", traceID),
			"correlationId": mergeTraceExpr("correlationId", correlationID),
			"parentEventId": mergeTraceExpr("parentEventId", parentEventID),
			"causalMs":      causalExpr,
		}}},
	}
	_, err := coll.UpdateOne(ctx, bson.M{"debounceKey": key}, pipeline, options.Update().SetUpsert(true))
	return err
}

func takeDueTrailingDocs(ctx context.Context, now time.Time, bucket string) ([]aidecisionmodels.TrailingDebounceSlot, error) {
	coll := trailingDebounceColl()
	if coll == nil {
		return nil, nil
	}
	nowMs := now.UnixMilli()
	var out []aidecisionmodels.TrailingDebounceSlot
	opts := options.FindOneAndDelete().SetSort(bson.D{{Key: "dueAtMs", Value: 1}})
	for {
		var doc aidecisionmodels.TrailingDebounceSlot
		err := coll.FindOneAndDelete(ctx, bson.M{
			"bucket":  bucket,
			"dueAtMs": bson.M{"$lte": nowMs},
		}, opts).Decode(&doc)
		if err == mongo.ErrNoDocuments {
			break
		}
		if err != nil {
			return out, err
		}
		out = append(out, doc)
	}
	return out, nil
}
