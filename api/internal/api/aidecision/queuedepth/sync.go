// Package queuedepth — độ sâu decision_events_queue trong RAM + đồng bộ Mongo.
// Tách khỏi decisionlive để crmqueue/worker gọi RefreshOrg mà không vướng import cycle (decisionlive → worker → crmqueue).
package queuedepth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/global"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// NormalizeOrgHex — khóa org trong mem (tránh lệch hoa/thường với aggregate Mongo).
func NormalizeOrgHex(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

type depthDoc struct {
	Pending         int64
	Leased          int64
	Processing      int64
	FailedRetryable int64
	FailedTerminal  int64
	Deferred        int64
	OtherActive     int64
	ReconciledAtMs  int64
}

type memQueueSnap struct {
	mu             sync.Mutex
	depth          map[string]int64
	reconciledAtMs int64
}

var memQueueDepth sync.Map // orgHex -> *memQueueSnap

func int64MapsEqual(a, b map[string]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		if b[k] != va {
			return false
		}
	}
	return true
}

func applyMemoryStore(orgHex string, doc depthDoc) {
	orgHex = NormalizeOrgHex(orgHex)
	newDepth := map[string]int64{
		"pending":          doc.Pending,
		"leased":           doc.Leased,
		"processing":       doc.Processing,
		"failed_retryable": doc.FailedRetryable,
		"failed_terminal":  doc.FailedTerminal,
		"deferred":         doc.Deferred,
		"other_active":     doc.OtherActive,
		"in_flight":        doc.Leased + doc.Processing,
	}
	v, _ := memQueueDepth.LoadOrStore(orgHex, &memQueueSnap{depth: make(map[string]int64)})
	s := v.(*memQueueSnap)
	s.mu.Lock()
	changed := s.depth == nil || !int64MapsEqual(s.depth, newDepth)
	s.depth = newDepth
	s.reconciledAtMs = doc.ReconciledAtMs
	s.mu.Unlock()

	if changed && metricsChangeLogEnabled() {
		logrus.WithFields(logrus.Fields{
			"orgHex":           orgHex,
			"pending":          doc.Pending,
			"leased":           doc.Leased,
			"processing":       doc.Processing,
			"failed_retryable": doc.FailedRetryable,
			"failed_terminal":  doc.FailedTerminal,
			"deferred":         doc.Deferred,
			"other_active":     doc.OtherActive,
			"in_flight":        doc.Leased + doc.Processing,
			"reconciledAtMs":   doc.ReconciledAtMs,
		}).Info("AI Decision metrics: queueDepth RAM thay đổi sau đồng bộ Mongo (reconcile)")
	}
}

func metricsChangeLogEnabled() bool {
	return strings.TrimSpace(os.Getenv("AI_DECISION_METRICS_CHANGE_LOG")) == "1"
}

// MemSnapshotForOrg đọc snapshot depth đã lưu trong RAM (sau reconcile / refresh).
func MemSnapshotForOrg(orgHex string) (depth map[string]int64, reconciledAt int64, ok bool) {
	orgHex = NormalizeOrgHex(orgHex)
	v, ok := memQueueDepth.Load(orgHex)
	if !ok {
		return nil, 0, false
	}
	s := v.(*memQueueSnap)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.depth == nil {
		return nil, s.reconciledAtMs, true
	}
	cp := make(map[string]int64, len(s.depth))
	for k, n := range s.depth {
		cp[k] = n
	}
	return cp, s.reconciledAtMs, true
}

// RefreshOrg đếm lại Mongo cho một org → ghi RAM (sau emit / lease / complete / fail).
func RefreshOrg(ctx context.Context, ownerOrgID primitive.ObjectID) error {
	if ownerOrgID.IsZero() {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	doc, err := oneOrgFromMongo(cctx, ownerOrgID)
	if err != nil {
		logrus.WithError(err).WithField("orgHex", ownerOrgID.Hex()).Debug("AI Decision: refresh queue depth theo org thất bại")
		return err
	}
	applyMemoryStore(NormalizeOrgHex(ownerOrgID.Hex()), doc)
	return nil
}

func matchDecisionQueueOwner(ownerOrgID primitive.ObjectID) bson.D {
	hex := ownerOrgID.Hex()
	return bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: "ownerOrganizationId", Value: ownerOrgID}},
		bson.D{{Key: "ownerOrganizationId", Value: hex}},
		bson.D{{Key: "orgId", Value: hex}},
	}}}
}

func decisionQueueStatusKeyFromBSON(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func normalizeDecisionQueueStatus(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "", "pending", "pend", "queued":
		return aidecisionmodels.EventStatusPending
	case "leased", "lease":
		return aidecisionmodels.EventStatusLeased
	case "processing", "process", "running":
		return aidecisionmodels.EventStatusProcessing
	case "failed_retryable", "failed-retryable", "retryable", "failedretryable":
		return aidecisionmodels.EventStatusFailedRetryable
	case "failed_terminal", "failed-terminal", "terminal", "failedterminal":
		return aidecisionmodels.EventStatusFailedTerminal
	case "deferred":
		return aidecisionmodels.EventStatusDeferred
	case "completed", "complete", "done", "success":
		return aidecisionmodels.EventStatusCompleted
	case "completed_no_handler", "completed-no-handler", "no_handler":
		return aidecisionmodels.EventStatusCompletedNoHandler
	case "completed_routing_skipped", "completed-routing-skipped", "routing_skipped":
		return aidecisionmodels.EventStatusCompletedRoutingSkipped
	default:
		return s
	}
}

// isTerminalCompletedQueueStatus — job đã ra khỏi backlog (không còn pending/leased/...).
func isTerminalCompletedQueueStatus(norm string) bool {
	switch norm {
	case aidecisionmodels.EventStatusCompleted,
		aidecisionmodels.EventStatusCompletedNoHandler,
		aidecisionmodels.EventStatusCompletedRoutingSkipped:
		return true
	default:
		return false
	}
}

func mapNormalizedDepthToDoc(depth map[string]int64, reconciledAt int64) depthDoc {
	var out depthDoc
	out.ReconciledAtMs = reconciledAt
	for st, c := range depth {
		switch st {
		case aidecisionmodels.EventStatusPending:
			out.Pending += c
		case aidecisionmodels.EventStatusLeased:
			out.Leased += c
		case aidecisionmodels.EventStatusProcessing:
			out.Processing += c
		case aidecisionmodels.EventStatusFailedRetryable:
			out.FailedRetryable += c
		case aidecisionmodels.EventStatusFailedTerminal:
			out.FailedTerminal += c
		case aidecisionmodels.EventStatusDeferred:
			out.Deferred += c
		default:
			out.OtherActive += c
		}
	}
	return out
}

type queueStatusAggRow struct {
	ID interface{} `bson:"_id"`
	C  int64       `bson:"c"`
}

type queueReconcileGroupID struct {
	OrgKey string      `bson:"orgKey"`
	St     interface{} `bson:"st"`
}

type queueReconcileAggRow struct {
	ID queueReconcileGroupID `bson:"_id"`
	C  int64                 `bson:"c"`
}

func oneOrgFromMongo(ctx context.Context, ownerOrgID primitive.ObjectID) (depthDoc, error) {
	var out depthDoc
	if ownerOrgID.IsZero() {
		return out, errors.New("ownerOrganizationId rỗng")
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionEventsQueue)
	if !ok || coll == nil {
		return out, errors.New("collection decision_events_queue chưa đăng ký")
	}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: matchDecisionQueueOwner(ownerOrgID)}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$status"},
			{Key: "c", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	}
	cur, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return out, err
	}
	defer cur.Close(ctx)
	normDepth := make(map[string]int64)
	for cur.Next(ctx) {
		var row queueStatusAggRow
		if err := cur.Decode(&row); err != nil {
			continue
		}
		raw := decisionQueueStatusKeyFromBSON(row.ID)
		norm := normalizeDecisionQueueStatus(raw)
		if isTerminalCompletedQueueStatus(norm) {
			continue
		}
		normDepth[norm] += row.C
	}
	if err := cur.Err(); err != nil {
		return out, err
	}
	return mapNormalizedDepthToDoc(normDepth, time.Now().UnixMilli()), nil
}

func pipelineReconcileQueueDepth() mongo.Pipeline {
	return mongo.Pipeline{
		bson.D{{Key: "$addFields", Value: bson.D{
			{Key: "queueOrgKey", Value: bson.D{
				{Key: "$cond", Value: bson.A{
					bson.D{{Key: "$eq", Value: bson.A{
						bson.D{{Key: "$type", Value: "$ownerOrganizationId"}},
						"objectId",
					}}},
					bson.D{{Key: "$toString", Value: "$ownerOrganizationId"}},
					bson.D{{Key: "$ifNull", Value: bson.A{"$ownerOrganizationId", "$orgId"}}},
				}},
			}},
		}}},
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "queueOrgKey", Value: bson.D{
				{Key: "$exists", Value: true},
				{Key: "$ne", Value: ""},
			}},
		}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "orgKey", Value: "$queueOrgKey"},
				{Key: "st", Value: "$status"},
			}},
			{Key: "c", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	}
}

// ReconcileAllFromMongo gom đếm theo org + status (toàn DB) → RAM.
func ReconcileAllFromMongo(ctx context.Context) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionEventsQueue)
	if !ok || coll == nil {
		return nil
	}
	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cur, err := coll.Aggregate(cctx, pipelineReconcileQueueDepth())
	if err != nil {
		return err
	}
	defer cur.Close(cctx)

	perOrg := make(map[string]map[string]int64)
	for cur.Next(cctx) {
		var row queueReconcileAggRow
		if err := cur.Decode(&row); err != nil {
			logrus.WithError(err).Warn("AI Decision reconcile: decode aggregate thất bại")
			continue
		}
		orgKey := NormalizeOrgHex(row.ID.OrgKey)
		if orgKey == "" {
			continue
		}
		norm := normalizeDecisionQueueStatus(decisionQueueStatusKeyFromBSON(row.ID.St))
		if isTerminalCompletedQueueStatus(norm) {
			continue
		}
		if perOrg[orgKey] == nil {
			perOrg[orgKey] = make(map[string]int64)
		}
		perOrg[orgKey][norm] += row.C
	}

	reconciledAt := time.Now().UnixMilli()
	for orgHex, depth := range perOrg {
		doc := mapNormalizedDepthToDoc(depth, reconciledAt)
		applyMemoryStore(orgHex, doc)
	}

	logrus.WithField("orgs", len(perOrg)).Info("AI Decision command center: đã reconcile độ sâu queue → RAM (ghi đè từ Mongo)")
	return cur.Err()
}

// StartBackground reconcile Mongo → RAM lần đầu đồng bộ, sau đó ticker (mặc định 5 phút, AI_DECISION_METRICS_RECONCILE_SEC).
func StartBackground(ctx context.Context) {
	interval := 5 * time.Minute
	if v := strings.TrimSpace(os.Getenv("AI_DECISION_METRICS_RECONCILE_SEC")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			interval = time.Duration(n) * time.Second
		}
	}
	base := context.Background()
	if err := ReconcileAllFromMongo(base); err != nil {
		logrus.WithError(err).Warn("AI Decision command center: reconcile lần đầu thất bại (RAM queue depth có thể rỗng đến lần sau)")
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := ReconcileAllFromMongo(base); err != nil {
					logrus.WithError(err).Warn("AI Decision command center: reconcile định kỳ thất bại")
				}
			}
		}
	}()
}
