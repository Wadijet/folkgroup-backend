// Package learningsvc — Service cho Learning engine.
//
// Learning engine là bộ nhớ học tập (learning memory) cho AI Commerce.
// Lưu trữ learning case đã hoàn thành — khác với Activity Log (event stream).
package learningsvc

import (
	"context"
	"fmt"
	"time"

	"meta_commerce/internal/api/learning/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	pkgapproval "meta_commerce/pkg/approval"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// LearningCaseService service CRUD cho learning_cases.
type LearningCaseService struct{}

// NewLearningCaseService tạo service.
func NewLearningCaseService() *LearningCaseService {
	return &LearningCaseService{}
}

func (s *LearningCaseService) getColl() (*mongo.Collection, error) {
	// Dùng learning_cases (PLATFORM_L1). decision_cases deprecated, migration script copy data nếu cần.
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.LearningCases)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.LearningCases, common.ErrNotFound)
	}
	return coll, nil
}

// CreateLearningCaseFromAction build LearningCase từ ActionPending và lưu.
// Gọi khi ActionPending đóng vòng đời — từ worker (executed/failed) hoặc handler (rejected).
// Supplement §7.2: không ghi learning đầy đủ khi decision case đóng timeout/manual/proposed (trừ khi env tắt skip).
func CreateLearningCaseFromAction(ctx context.Context, ap *pkgapproval.ActionPending) (*models.LearningCase, error) {
	if ap == nil {
		return nil, nil
	}
	lc, err := BuildLearningCaseFromAction(ctx, ap)
	if err != nil {
		return nil, err
	}
	if shouldSkipLearningForDecisionClosure(lc.DecisionCaseClosureType) {
		return nil, nil
	}
	svc := NewLearningCaseService()
	return svc.CreateLearningCase(ctx, lc)
}

// CreateLearningCase tạo learning case mới.
func (s *LearningCaseService) CreateLearningCase(ctx context.Context, lc *models.LearningCase) (*models.LearningCase, error) {
	coll, err := s.getColl()
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	lc.CreatedAt = now
	if lc.ClosedAt == 0 {
		lc.ClosedAt = now
	}
	res, err := coll.InsertOne(ctx, lc)
	if err != nil {
		return nil, err
	}
	lc.ID = res.InsertedID.(primitive.ObjectID)
	return lc, nil
}

// FindLearningCaseById tìm case theo ID.
func (s *LearningCaseService) FindLearningCaseById(ctx context.Context, id primitive.ObjectID, ownerOrgID primitive.ObjectID) (*models.LearningCase, error) {
	coll, err := s.getColl()
	if err != nil {
		return nil, err
	}
	filter := bson.M{"_id": id, "ownerOrganizationId": ownerOrgID}
	var lc models.LearningCase
	err = coll.FindOne(ctx, filter).Decode(&lc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("không tìm thấy learning case: %w", common.ErrNotFound)
		}
		return nil, err
	}
	return &lc, nil
}

// ListLearningCases danh sách case với filter và pagination.
func (s *LearningCaseService) ListLearningCases(ctx context.Context, ownerOrgID primitive.ObjectID, filter bson.M, limit, skip int, sortField string, sortOrder int) ([]models.LearningCase, int64, error) {
	coll, err := s.getColl()
	if err != nil {
		return nil, 0, err
	}
	if filter == nil {
		filter = bson.M{}
	}
	filter["ownerOrganizationId"] = ownerOrgID

	total, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if sortField == "" {
		sortField = "createdAt"
	}
	if sortOrder == 0 {
		sortOrder = -1
	}

	opts := options.Find().SetSort(bson.D{{Key: sortField, Value: sortOrder}}).SetSkip(int64(skip)).SetLimit(int64(limit))
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var list []models.LearningCase
	if err := cursor.All(ctx, &list); err != nil {
		return nil, 0, err
	}
	if list == nil {
		list = []models.LearningCase{}
	}
	return list, total, nil
}
