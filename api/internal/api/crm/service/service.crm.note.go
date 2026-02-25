// Package crmvc - Service ghi chú CRM (crm_notes).
package crmvc

import (
	"context"
	"fmt"
	"time"

	crmdto "meta_commerce/internal/api/crm/dto"
	crmmodels "meta_commerce/internal/api/crm/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// CrmNoteService xử lý CRUD ghi chú khách.
type CrmNoteService struct {
	*basesvc.BaseServiceMongoImpl[crmmodels.CrmNote]
}

// NewCrmNoteService tạo CrmNoteService mới.
func NewCrmNoteService() (*CrmNoteService, error) {
	coll, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmNotes)
	if !exist {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.CrmNotes, common.ErrNotFound)
	}
	return &CrmNoteService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[crmmodels.CrmNote](coll),
	}, nil
}

// CreateNote tạo ghi chú mới.
func (s *CrmNoteService) CreateNote(ctx context.Context, input *crmdto.CrmNoteCreateInput, ownerOrgID, createdBy primitive.ObjectID) (*crmmodels.CrmNote, error) {
	now := time.Now().UnixMilli()
	doc := crmmodels.CrmNote{
		CustomerId:          input.CustomerId,
		OwnerOrganizationID: ownerOrgID,
		NoteText:            input.NoteText,
		NextAction:          input.NextAction,
		NextActionDate:      input.NextActionDate,
		CreatedBy:           createdBy,
		IsDeleted:           false,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	note, err := s.InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}
	return &note, nil
}

// FindByCustomerId trả về danh sách ghi chú của khách (chưa xóa, mới nhất trước).
func (s *CrmNoteService) FindByCustomerId(ctx context.Context, customerId string, ownerOrgID primitive.ObjectID, limit int) ([]crmmodels.CrmNote, error) {
	if limit <= 0 {
		limit = 50
	}
	filter := bson.M{
		"customerId":          customerId,
		"ownerOrganizationId": ownerOrgID,
		"isDeleted":           false,
	}
	opts := mongoopts.Find().SetLimit(int64(limit)).SetSort(bson.D{{Key: "createdAt", Value: -1}})
	return s.Find(ctx, filter, opts)
}

// SoftDelete đánh dấu ghi chú đã xóa.
func (s *CrmNoteService) SoftDelete(ctx context.Context, noteId primitive.ObjectID, ownerOrgID primitive.ObjectID) error {
	filter := bson.M{"_id": noteId, "ownerOrganizationId": ownerOrgID}
	update := bson.M{"$set": bson.M{"isDeleted": true, "updatedAt": time.Now().UnixMilli()}}
	_, err := s.UpdateOne(ctx, filter, update, nil)
	return err
}
