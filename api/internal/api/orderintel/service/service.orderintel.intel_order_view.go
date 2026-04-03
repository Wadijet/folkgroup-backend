// Package orderintelsvc — View tối thiểu để tính Order Intelligence từ commerce_orders hoặc fallback pc_pos_orders.
package orderintelsvc

import (
	"strings"

	ordermodels "meta_commerce/internal/api/order/models"
	pcmodels "meta_commerce/internal/api/pc/models"

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
	// ID — _id bản ghi dùng làm nguồn tính intel (commerce_orders khi có, không thì pc_pos_orders).
	ID primitive.ObjectID
	// CommerceMongoID — _id commerce_orders khi nguồn là canonical (để trace).
	CommerceMongoID primitive.ObjectID
	// PancakeSourceMongoID — _id pc_pos_orders (luôn set khi biết).
	PancakeSourceMongoID primitive.ObjectID
	Status           int
	InsertedAt       int64
	PosUpdatedAt     int64
	PosData          map[string]interface{}
	OwnerOrganizationID primitive.ObjectID
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
		Uid:                  strings.TrimSpace(c.Uid),
		PageId:               c.PageId,
		PostId:               c.PostId,
		CustomerId:           c.CustomerId,
		LinksCustomerUid:     linksCust,
		OrderId:              c.OrderId,
		ID:                   c.ID,
		CommerceMongoID:      c.ID,
		PancakeSourceMongoID: c.SourceRecordMongoID,
		Status:               c.Status,
		InsertedAt:           c.InsertedAt,
		PosUpdatedAt:         c.PosUpdatedAt,
		PosData:              c.PosData,
		OwnerOrganizationID:  c.OwnerOrganizationID,
	}
}

func newIntelViewFromPC(o *pcmodels.PcPosOrder) *intelOrderView {
	if o == nil {
		return nil
	}
	return &intelOrderView{
		Uid:                  strings.TrimSpace(o.Uid),
		PageId:               o.PageId,
		PostId:               o.PostId,
		CustomerId:           o.CustomerId,
		LinksCustomerUid:     strings.TrimSpace(o.LinksCustomerUid),
		OrderId:              o.OrderId,
		ID:                   o.ID,
		CommerceMongoID:      primitive.NilObjectID,
		PancakeSourceMongoID: o.ID,
		Status:               o.Status,
		InsertedAt:           o.InsertedAt,
		PosUpdatedAt:         o.PosUpdatedAt,
		PosData:              o.PosData,
		OwnerOrganizationID:  o.OwnerOrganizationID,
	}
}
