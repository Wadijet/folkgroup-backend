// Package orderintelsvc — View tối thiểu để tính Order Intelligence từ order_canonical (L2).
package orderintelsvc

import (
	"strings"

	ordermodels "meta_commerce/internal/api/order/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// intelOrderView dữ liệu tối thiểu cho ComputeSnapshot (không phụ thuộc trực tiếp vào một collection).
type intelOrderView struct {
	Uid              string
	PageId           string
	PostId           string
	CustomerId       string
	LinksCustomerUid string
	OrderId          int64
	// ID — _id bản ghi dùng làm nguồn tính intel (order_canonical).
	ID primitive.ObjectID
	// OrderCanonicalMongoID — _id order_canonical.
	OrderCanonicalMongoID primitive.ObjectID
	// PancakeSourceMongoID — _id bản ghi POS gốc (sourceRecordMongoID khi source là Pancake).
	PancakeSourceMongoID primitive.ObjectID
	Status                int
	InsertedAt            int64
	PosUpdatedAt          int64
	PosData               map[string]interface{}
	OwnerOrganizationID   primitive.ObjectID
}

func newIntelViewFromCommerce(c *ordermodels.CommerceOrder) *intelOrderView {
	if c == nil {
		return nil
	}
	linksCust := ""
	if c.Links != nil {
		if li, ok := c.Links["customer"]; ok {
			linksCust = strings.TrimSpace(li.Uid)
		}
	}
	return &intelOrderView{
		Uid:                   strings.TrimSpace(c.Uid),
		PageId:                c.PageId,
		PostId:                c.PostId,
		CustomerId:            c.CustomerId,
		LinksCustomerUid:      linksCust,
		OrderId:               c.OrderId,
		ID:                    c.ID,
		OrderCanonicalMongoID: c.ID,
		PancakeSourceMongoID:  c.SourceRecordMongoID,
		Status:                c.Status,
		InsertedAt:            c.InsertedAt,
		PosUpdatedAt:          c.PosUpdatedAt,
		PosData:               c.PosData,
		OwnerOrganizationID:   c.OwnerOrganizationID,
	}
}
