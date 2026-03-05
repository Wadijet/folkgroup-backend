// Package dto - DTO chung cho Meta sync-upsert.
package dto

// MetaSyncUpsertInput input chung cho sync-upsert: nhận raw Meta API response + ownerOrganizationId.
// metaData: dữ liệu gốc từ Meta API (campaign, adset, ad, insight...).
// ownerOrganizationId: tổ chức sở hữu (tự transform string → ObjectID).
type MetaSyncUpsertInput struct {
	MetaData            map[string]interface{} `json:"metaData" validate:"required"`
	OwnerOrganizationID string                 `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
}

// MetaAdInsightSyncUpsertInput input cho sync-upsert insight. Cần thêm objectId, objectType vì phụ thuộc level.
type MetaAdInsightSyncUpsertInput struct {
	MetaData            map[string]interface{} `json:"metaData" validate:"required"`
	OwnerOrganizationID string                 `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
	AdAccountId         string                 `json:"adAccountId" validate:"required"`
	ObjectId            string                 `json:"objectId" validate:"required"`
	ObjectType          string                 `json:"objectType" validate:"required"`
}
