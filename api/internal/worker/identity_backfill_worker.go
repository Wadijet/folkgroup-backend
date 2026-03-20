// Package worker — IdentityBackfillWorker backfill uid, sourceIds, links cho document cũ (4 lớp identity).
package worker

import (
	"context"
	"os"
	"strings"
	"time"

	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/utility"
	"meta_commerce/internal/utility/identity"
	"meta_commerce/internal/worker/metrics"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// IdentityBackfillMode chế độ backfill: uid, sourceIds, links, all.
const (
	IdentityBackfillModeUid       = "uid"
	IdentityBackfillModeSourceIds = "sourceIds"
	IdentityBackfillModeLinks    = "links"
	IdentityBackfillModeAll      = "all"
)

// IdentityBackfillWorker worker backfill identity 4 lớp cho doc cũ.
type IdentityBackfillWorker struct {
	interval  time.Duration
	batchSize int
}

// NewIdentityBackfillWorker tạo mới IdentityBackfillWorker.
func NewIdentityBackfillWorker(interval time.Duration, batchSize int) *IdentityBackfillWorker {
	if interval < time.Minute {
		interval = 5 * time.Minute
	}
	if batchSize <= 0 {
		batchSize = 500
	}
	return &IdentityBackfillWorker{
		interval:  interval,
		batchSize: batchSize,
	}
}

// Start chạy worker trong vòng lặp.
func (w *IdentityBackfillWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	mode := getIdentityBackfillMode()

	log.WithFields(map[string]interface{}{
		"interval":  w.interval.String(),
		"batchSize": w.batchSize,
		"mode":      mode,
	}).Info("🔗 [IDENTITY_BACKFILL] Starting Identity Backfill Worker...")

	for {
		interval, batchSize := GetEffectiveWorkerSchedule(WorkerIdentityBackfill, w.interval, w.batchSize)

		if !IsWorkerActive(WorkerIdentityBackfill) {
			select {
			case <-ctx.Done():
				log.Info("🔗 [IDENTITY_BACKFILL] Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}

		p := GetPriority(WorkerIdentityBackfill, PriorityLowest)
		if ShouldThrottle(p) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}

		effInterval := GetEffectiveInterval(interval, p)
		effBatchSize := GetEffectiveBatchSize(batchSize, p)
		if effBatchSize < 1 {
			effBatchSize = 1
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{"panic": r}).Error("🔗 [IDENTITY_BACKFILL] Panic khi xử lý")
				}
			}()

			start := time.Now()
			totalProcessed, totalUpdated := w.runBackfill(ctx, mode, effBatchSize)
			metrics.RecordDuration("identity_backfill:"+mode, time.Since(start))

			if totalProcessed > 0 || totalUpdated > 0 {
				log.WithFields(map[string]interface{}{
					"mode":     mode,
					"processed": totalProcessed,
					"updated":  totalUpdated,
				}).Info("🔗 [IDENTITY_BACKFILL] Chu kỳ hoàn thành")
			}
		}()

		select {
		case <-ctx.Done():
			return
		case <-time.After(effInterval):
		}
	}
}

func getIdentityBackfillMode() string {
	mode := strings.TrimSpace(strings.ToLower(os.Getenv("WORKER_IDENTITY_BACKFILL_MODE")))
	if mode == "" {
		mode = IdentityBackfillModeUid
	}
	switch mode {
	case IdentityBackfillModeUid, IdentityBackfillModeSourceIds, IdentityBackfillModeLinks, IdentityBackfillModeAll:
		return mode
	default:
		return IdentityBackfillModeUid
	}
}

func (w *IdentityBackfillWorker) runBackfill(ctx context.Context, mode string, batchSize int) (totalProcessed, totalUpdated int) {
	collections := identity.GetAllEnrichedCollectionNames()
	for _, collName := range collections {
		coll, ok := global.RegistryCollections.Get(collName)
		if !ok || coll == nil {
			continue
		}

		switch mode {
		case IdentityBackfillModeUid:
			p, u := w.backfillUid(ctx, coll, collName, batchSize)
			totalProcessed += p
			totalUpdated += u
		case IdentityBackfillModeSourceIds:
			p, u := w.backfillSourceIds(ctx, coll, collName, batchSize)
			totalProcessed += p
			totalUpdated += u
		case IdentityBackfillModeLinks:
			p, u := w.backfillLinks(ctx, coll, collName, batchSize)
			totalProcessed += p
			totalUpdated += u
		case IdentityBackfillModeAll:
			p1, u1 := w.backfillUid(ctx, coll, collName, batchSize)
			p2, u2 := w.backfillSourceIds(ctx, coll, collName, batchSize)
			p3, u3 := w.backfillLinks(ctx, coll, collName, batchSize)
			totalProcessed += p1 + p2 + p3
			totalUpdated += u1 + u2 + u3
		}
	}
	return totalProcessed, totalUpdated
}

// backfillUid set uid = prefix + _id.Hex() cho doc thiếu uid.
func (w *IdentityBackfillWorker) backfillUid(ctx context.Context, coll *mongo.Collection, collName string, batchSize int) (processed, updated int) {
	cfg, ok := identity.GetConfig(collName)
	if !ok {
		return 0, 0
	}

	filter := bson.M{
		"$or": []bson.M{
			{"uid": bson.M{"$exists": false}},
			{"uid": ""},
		},
	}
	opts := options.Find().SetLimit(int64(batchSize)).SetProjection(bson.M{"_id": 1})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return 0, 0
	}
	defer cursor.Close(ctx)

	var ids []primitive.ObjectID
	for cursor.Next(ctx) {
		var doc struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		ids = append(ids, doc.ID)
		processed++
	}
	if len(ids) == 0 {
		return processed, 0
	}

	// Update từng doc với uid = prefix + _id.Hex()
	for _, id := range ids {
		uidVal := utility.UIDFromObjectID(cfg.Prefix, id)
		_, err := coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"uid": uidVal}})
		if err == nil {
			updated++
		}
	}
	return processed, updated
}

// backfillSourceIds điền sourceIds từ path cho doc thiếu.
func (w *IdentityBackfillWorker) backfillSourceIds(ctx context.Context, coll *mongo.Collection, collName string, batchSize int) (processed, updated int) {
	cfg, ok := identity.GetConfig(collName)
	if !ok || len(cfg.SourceKeys) == 0 {
		return 0, 0
	}

	// Chỉ lấy doc thiếu ít nhất một source key cần thiết
	orConditions := []bson.M{
		{"sourceIds": bson.M{"$exists": false}},
		{"sourceIds": nil},
	}
	for _, sk := range cfg.SourceKeys {
		orConditions = append(orConditions, bson.M{"sourceIds." + sk.Source: bson.M{"$exists": false}})
	}
	filter := bson.M{"$or": orConditions}
	opts := options.Find().SetLimit(int64(batchSize))
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return 0, 0
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc map[string]interface{}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		processed++

		sourceIds := make(map[string]interface{})
		if existing, ok := doc["sourceIds"].(map[string]interface{}); ok {
			for k, v := range existing {
				sourceIds[k] = v
			}
		}
		needsUpdate := false
		for _, sk := range cfg.SourceKeys {
			if _, has := sourceIds[sk.Source]; has {
				continue
			}
			v := getMapValueByPath(doc, sk.Path)
			if v != nil {
				s := toStringVal(v)
				if s != "" {
					sourceIds[sk.Source] = s
					needsUpdate = true
				}
			}
		}
		if needsUpdate {
			id, ok := doc["_id"]
			if !ok {
				continue
			}
			_, err := coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"sourceIds": sourceIds}})
			if err == nil {
				updated++
			}
		}
	}
	return processed, updated
}

// backfillLinks điền links từ path, resolve qua CRM.
func (w *IdentityBackfillWorker) backfillLinks(ctx context.Context, coll *mongo.Collection, collName string, batchSize int) (processed, updated int) {
	cfg, ok := identity.GetConfig(collName)
	if !ok || len(cfg.LinkKeys) == 0 {
		return 0, 0
	}

	// Chỉ lấy doc thiếu links hoặc thiếu ít nhất một link key
	orConditions := []bson.M{
		{"links": bson.M{"$exists": false}},
		{"links": nil},
	}
	for _, lk := range cfg.LinkKeys {
		orConditions = append(orConditions, bson.M{"links." + lk.Key: bson.M{"$exists": false}})
	}
	filter := bson.M{"$or": orConditions}
	opts := options.Find().SetLimit(int64(batchSize))
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return 0, 0
	}
	defer cursor.Close(ctx)

	resolver := identity.GetDefaultResolver()

	for cursor.Next(ctx) {
		var doc map[string]interface{}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		processed++

		// Kiểm tra đã có đủ links chưa
		links, _ := doc["links"].(map[string]interface{})
		if links == nil {
			links = make(map[string]interface{})
		}
		ownerOrgID := getOwnerOrgIDFromMap(doc)

		needsUpdate := false
		for _, lk := range cfg.LinkKeys {
			if _, has := links[lk.Key]; has {
				continue
			}
			val := getMapValueByPath(doc, lk.Path)
			if val == nil {
				continue
			}
			extId := toStringVal(val)
			if extId == "" {
				continue
			}

			if utility.IsUID(extId) {
				links[lk.Key] = map[string]interface{}{
					"uid":          extId,
					"externalRefs": []interface{}{},
					"status":       identity.LinkStatusResolved,
				}
				needsUpdate = true
				continue
			}

			if resolver != nil && ownerOrgID != primitive.NilObjectID {
				if resolvedUid, ok := resolver.ResolveToUid(ctx, extId, lk.Source, ownerOrgID); ok && resolvedUid != "" {
					links[lk.Key] = map[string]interface{}{
						"uid":          resolvedUid,
						"externalRefs": []interface{}{map[string]interface{}{"source": lk.Source, "id": extId}},
						"status":       identity.LinkStatusResolved,
					}
					needsUpdate = true
					continue
				}
			}

			src := lk.Source
			if src == "" {
				src = "unknown"
			}
			links[lk.Key] = map[string]interface{}{
				"uid":          "",
				"externalRefs": []interface{}{map[string]interface{}{"source": src, "id": extId}},
				"status":       identity.LinkStatusPendingResolution,
			}
			needsUpdate = true
		}

		if needsUpdate {
			id := doc["_id"]
			_, err := coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"links": links}})
			if err == nil {
				updated++
			}
		}
	}
	return processed, updated
}

func getMapValueByPath(m map[string]interface{}, path string) interface{} {
	return identity.GetMapValueByPath(m, path)
}

func toStringVal(v interface{}) string {
	return identity.ToString(v)
}

func getOwnerOrgIDFromMap(doc map[string]interface{}) primitive.ObjectID {
	v := getMapValueByPath(doc, "ownerOrganizationId")
	if v == nil {
		return primitive.NilObjectID
	}
	switch x := v.(type) {
	case primitive.ObjectID:
		return x
	case *primitive.ObjectID:
		if x != nil {
			return *x
		}
		return primitive.NilObjectID
	case string:
		oid, _ := primitive.ObjectIDFromHex(x)
		return oid
	default:
		return primitive.NilObjectID
	}
}
