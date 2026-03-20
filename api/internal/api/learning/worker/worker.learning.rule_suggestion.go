// Package worker — LearningRuleSuggestionWorker: phân tích learning_cases, tạo rule suggestions (Phase 3).
package worker

import (
	"context"
	"os"
	"strings"
	"time"

	learningsvc "meta_commerce/internal/api/learning/service"
	"github.com/sirupsen/logrus"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/global"
)

// LearningRuleSuggestionWorker định kỳ phân tích learning_cases theo org, tạo RuleSuggestion.
type LearningRuleSuggestionWorker struct {
	interval time.Duration
}

// NewLearningRuleSuggestionWorker tạo mới.
func NewLearningRuleSuggestionWorker(interval time.Duration) *LearningRuleSuggestionWorker {
	if interval < 1*time.Hour {
		interval = 1 * time.Hour
	}
	return &LearningRuleSuggestionWorker{interval: interval}
}

// Start chạy worker.
func (w *LearningRuleSuggestionWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	log.WithField("interval", w.interval.String()).Info("📋 [LEARNING_RULE_SUGGESTION] Starting Rule Suggestion Worker...")

	for {
		if !worker.IsWorkerActive(worker.WorkerLearningRuleSuggestion) {
			select {
			case <-ctx.Done():
				log.Info("📋 [LEARNING_RULE_SUGGESTION] Worker stopped")
				return
			case <-time.After(5 * time.Minute):
			}
			continue
		}

		interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerLearningRuleSuggestion, w.interval, 1)
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📋 [LEARNING_RULE_SUGGESTION] Panic")
				}
			}()

			runRuleSuggestion(ctx, log)
		}()
	}
}

func runRuleSuggestion(ctx context.Context, log *logrus.Logger) {
	// Chỉ chạy khi bật env
	if strings.TrimSpace(strings.ToLower(os.Getenv("LEARNING_RULE_SUGGESTION_ENABLED"))) != "true" {
		return
	}

	orgColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.Organizations)
	if !ok {
		return
	}
	cursor, err := orgColl.Find(ctx, bson.M{}, nil)
	if err != nil {
		return
	}
	defer cursor.Close(ctx)

	var orgIDs []primitive.ObjectID
	for cursor.Next(ctx) {
		var doc struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		orgIDs = append(orgIDs, doc.ID)
	}
	if err := cursor.Err(); err != nil {
		return
	}

	seen := make(map[string]bool)
	totalCreated := 0
	for _, oid := range orgIDs {
		if oid.IsZero() {
			continue
		}
		key := oid.Hex()
		if seen[key] {
			continue
		}
		seen[key] = true
		n, err := learningsvc.AnalyzeAndSuggestRules(ctx, oid)
		if err != nil {
			log.WithError(err).WithField("orgId", key).Warn("📋 [LEARNING_RULE_SUGGESTION] Phân tích thất bại")
			continue
		}
		if n > 0 {
			totalCreated += n
			log.WithFields(map[string]interface{}{"orgId": key, "created": n}).Info("📋 [LEARNING_RULE_SUGGESTION] Đã tạo rule suggestions")
		}
	}
}
