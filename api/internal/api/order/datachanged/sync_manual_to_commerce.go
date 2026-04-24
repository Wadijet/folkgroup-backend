// Đồng bộ đơn canonical order_core_records từ L1 mirror nhập tay (order_src_manual_orders), không phát EmitDataChanged (tránh lặp queue AID).
package orderdatachanged

import (
	"context"
	"time"

	"meta_commerce/internal/api/events"
	ordermodels "meta_commerce/internal/api/order/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SyncCommerceOrderFromManualDataChange upsert order_core_records từ event datachanged order_src_manual_orders.
func SyncCommerceOrderFromManualDataChange(ctx context.Context, e events.DataChangeEvent) error {
	if e.Document == nil {
		return nil
	}
	b, err := bson.Marshal(e.Document)
	if err != nil {
		return err
	}
	var row pcmodels.PcPosOrder
	if err := bson.Unmarshal(b, &row); err != nil {
		return err
	}
	return UpsertCommerceFromManualMirrorOrder(ctx, &row)
}

// UpsertCommerceFromManualMirrorOrder ghi order_core_records theo một dòng L1 nhập tay (cùng model PcPosOrder để posData / CRM tương thích).
func UpsertCommerceFromManualMirrorOrder(ctx context.Context, row *pcmodels.PcPosOrder) error {
	if row == nil || row.OwnerOrganizationID.IsZero() || row.ID.IsZero() {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.OrderCanonical)
	if !ok || coll == nil {
		return nil
	}
	now := time.Now().UnixMilli()
	co := mapManualMirrorToCommerce(row, now)
	filter := bson.M{
		"ownerOrganizationId": row.OwnerOrganizationID,
		"source":              ordermodels.SourceManual,
		"sourceRecordMongoId": row.ID,
	}
	set := bson.M{
		"uid":                 co.Uid,
		"source":              co.Source,
		"sourceIds":           co.SourceIds,
		"links":               co.Links,
		"ownerOrganizationId": co.OwnerOrganizationID,
		"sourceRecordMongoId": co.SourceRecordMongoID,
		"orderId":             co.OrderId,
		"status":              co.Status,
		"insertedAt":          co.InsertedAt,
		"posUpdatedAt":        co.PosUpdatedAt,
		"pageId":              co.PageId,
		"postId":              co.PostId,
		"customerId":          co.CustomerId,
		"posData":             co.PosData,
		"updatedAt":           now,
	}
	update := mongo.NewUpdateOneModel().
		SetFilter(filter).
		SetUpdate(bson.M{
			"$set":         set,
			"$setOnInsert": bson.M{"createdAt": now},
		}).
		SetUpsert(true)
	_, err := coll.BulkWrite(ctx, []mongo.WriteModel{update}, options.BulkWrite().SetOrdered(true))
	return err
}

func mapManualMirrorToCommerce(row *pcmodels.PcPosOrder, nowMs int64) *ordermodels.CommerceOrder {
	co := mapPancakeToCommerce(row, nowMs)
	co.Source = ordermodels.SourceManual
	if co.SourceIds == nil {
		co.SourceIds = map[string]string{}
	}
	if row.OrderId == 0 && !row.ID.IsZero() {
		co.SourceIds["manual_row"] = row.ID.Hex()
	}
	return co
}
