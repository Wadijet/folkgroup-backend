// Package decisionsvc — Service cho Decision Brain.
//
// Decision Brain là bộ nhớ học tập (learning memory) cho AI Commerce.
// Lưu trữ decision case đã hoàn thành — khác với Activity Log (event stream).
// Case chỉ tạo khi entity nguồn đã đóng vòng đời.
package decisionsvc

import (
	"context"
	"fmt"
	"time"

	"meta_commerce/internal/api/decision/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DecisionCaseService service CRUD cho decision_cases.
type DecisionCaseService struct{}

// NewDecisionCaseService tạo service.
func NewDecisionCaseService() *DecisionCaseService {
	return &DecisionCaseService{}
}

// getColl trả về collection decision_cases.
func (s *DecisionCaseService) getColl() (*mongo.Collection, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCases)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.DecisionCases, common.ErrNotFound)
	}
	return coll, nil
}

// CreateDecisionCase tạo decision case mới.
func (s *DecisionCaseService) CreateDecisionCase(ctx context.Context, dc *models.DecisionCase) (*models.DecisionCase, error) {
	coll, err := s.getColl()
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	dc.CreatedAt = now
	dc.UpdatedAt = now
	res, err := coll.InsertOne(ctx, dc)
	if err != nil {
		return nil, err
	}
	dc.ID = res.InsertedID.(primitive.ObjectID)
	return dc, nil
}

// FindDecisionCaseById tìm case theo ID.
func (s *DecisionCaseService) FindDecisionCaseById(ctx context.Context, id primitive.ObjectID, ownerOrgID primitive.ObjectID) (*models.DecisionCase, error) {
	coll, err := s.getColl()
	if err != nil {
		return nil, err
	}
	filter := bson.M{"_id": id, "ownerOrganizationId": ownerOrgID}
	var dc models.DecisionCase
	err = coll.FindOne(ctx, filter).Decode(&dc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("không tìm thấy decision case: %w", common.ErrNotFound)
		}
		return nil, err
	}
	return &dc, nil
}

// FindDecisionCaseByCaseId tìm case theo caseId (business ID).
func (s *DecisionCaseService) FindDecisionCaseByCaseId(ctx context.Context, caseId string, ownerOrgID primitive.ObjectID) (*models.DecisionCase, error) {
	coll, err := s.getColl()
	if err != nil {
		return nil, err
	}
	filter := bson.M{"caseId": caseId, "ownerOrganizationId": ownerOrgID}
	var dc models.DecisionCase
	err = coll.FindOne(ctx, filter).Decode(&dc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("không tìm thấy decision case: %w", common.ErrNotFound)
		}
		return nil, err
	}
	return &dc, nil
}

// ListDecisionCases danh sách case với filter và pagination.
func (s *DecisionCaseService) ListDecisionCases(ctx context.Context, ownerOrgID primitive.ObjectID, filter bson.M, limit, skip int, sortField string, sortOrder int) ([]models.DecisionCase, int64, error) {
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

	var list []models.DecisionCase
	if err := cursor.All(ctx, &list); err != nil {
		return nil, 0, err
	}
	if list == nil {
		list = []models.DecisionCase{}
	}
	return list, total, nil
}

// QueryDecisionCasesByTarget query theo targetType + targetId.
func (s *DecisionCaseService) QueryDecisionCasesByTarget(ctx context.Context, ownerOrgID primitive.ObjectID, targetType, targetId string, limit int) ([]models.DecisionCase, error) {
	filter := bson.M{}
	if targetType != "" {
		filter["targetType"] = targetType
	}
	if targetId != "" {
		filter["targetId"] = targetId
	}
	list, _, err := s.ListDecisionCases(ctx, ownerOrgID, filter, limit, 0, "sourceClosedAt", -1)
	return list, err
}

// QueryDecisionCasesByCaseType query theo caseType.
func (s *DecisionCaseService) QueryDecisionCasesByCaseType(ctx context.Context, ownerOrgID primitive.ObjectID, caseType string, limit int) ([]models.DecisionCase, error) {
	filter := bson.M{"caseType": caseType}
	list, _, err := s.ListDecisionCases(ctx, ownerOrgID, filter, limit, 0, "sourceClosedAt", -1)
	return list, err
}

// QueryDecisionCasesByCategory query theo caseCategory.
func (s *DecisionCaseService) QueryDecisionCasesByCategory(ctx context.Context, ownerOrgID primitive.ObjectID, caseCategory string, limit int) ([]models.DecisionCase, error) {
	filter := bson.M{"caseCategory": caseCategory}
	list, _, err := s.ListDecisionCases(ctx, ownerOrgID, filter, limit, 0, "sourceClosedAt", -1)
	return list, err
}

// QueryDecisionCasesByGoal query theo goalCode.
func (s *DecisionCaseService) QueryDecisionCasesByGoal(ctx context.Context, ownerOrgID primitive.ObjectID, goalCode string, limit int) ([]models.DecisionCase, error) {
	filter := bson.M{"goalCode": goalCode}
	list, _, err := s.ListDecisionCases(ctx, ownerOrgID, filter, limit, 0, "sourceClosedAt", -1)
	return list, err
}

// QueryDecisionCasesByResult query theo result.
func (s *DecisionCaseService) QueryDecisionCasesByResult(ctx context.Context, ownerOrgID primitive.ObjectID, result string, limit int) ([]models.DecisionCase, error) {
	filter := bson.M{"result": result}
	list, _, err := s.ListDecisionCases(ctx, ownerOrgID, filter, limit, 0, "sourceClosedAt", -1)
	return list, err
}
