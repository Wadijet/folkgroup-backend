package identity

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Đảm bảo key registry không phụ thuộc global.MongoDB_ColNames lúc package init (luôn "" trước InitGlobal).
func TestShouldEnrichPcPosOrdersKhongCanInitGlobal(t *testing.T) {
	if !ShouldEnrich("pc_pos_orders") {
		t.Fatal("ShouldEnrich(pc_pos_orders) phải true ngay khi load package")
	}
	if _, ok := GetConfig("pc_pos_orders"); !ok {
		t.Fatal("GetConfig(pc_pos_orders) phải ok")
	}
}

func TestShouldEnrichOrderCanonicalKhongCanInitGlobal(t *testing.T) {
	if !ShouldEnrich("order_canonical") {
		t.Fatal("ShouldEnrich(order_canonical) phải true ngay khi load package")
	}
	if _, ok := GetConfig("order_canonical"); !ok {
		t.Fatal("GetConfig(order_canonical) phải ok")
	}
}

// TestEnrichPcPosCustomersBonTangIdentity — đủ 4 lớp: _id, uid, sourceIds (pos + pancake_customer + facebook), links.shop pending.
func TestEnrichPcPosCustomersBonTangIdentity(t *testing.T) {
	oid := primitive.NewObjectID()
	doc := map[string]interface{}{
		"_id":                 oid,
		"ownerOrganizationId":   oid, // chỉ cần non-zero cho flow enrich
		"shopId":              int64(860225178),
		"customerId":          "9bef52dd-b3e9-4b58-9c1e-1f977f23f1ec",
		"posData": map[string]interface{}{
			"id":          "9bef52dd-b3e9-4b58-9c1e-1f977f23f1ec",
			"customer_id": "c5ea2018-1d27-4e47-be6d-8096d18c01ac",
			"fb_id":       "109003588125335_26512669371686543",
		},
	}
	if err := EnrichIdentity4Layers(context.Background(), "pc_pos_customers", doc, nil); err != nil {
		t.Fatal(err)
	}
	sids, _ := doc["sourceIds"].(map[string]interface{})
	if sids["pos"] != "9bef52dd-b3e9-4b58-9c1e-1f977f23f1ec" {
		t.Fatalf("sourceIds.pos = %v", sids["pos"])
	}
	if sids["pancake_customer"] != "c5ea2018-1d27-4e47-be6d-8096d18c01ac" {
		t.Fatalf("sourceIds.pancake_customer = %v", sids["pancake_customer"])
	}
	if sids["facebook"] != "109003588125335_26512669371686543" {
		t.Fatalf("sourceIds.facebook = %v", sids["facebook"])
	}
	links, _ := doc["links"].(map[string]interface{})
	shop, _ := links["shop"].(map[string]interface{})
	if shop["status"] != LinkStatusPendingResolution {
		t.Fatalf("links.shop.status = %v", shop["status"])
	}
}
