// Package layer3 - Unit test cho DeriveFromNested và DeriveFromMap.
package layer3

import (
	"testing"
	"time"
)

func TestDeriveFromNested_NilMap_ReturnsNil(t *testing.T) {
	result := DeriveFromNested(nil, time.Now().UnixMilli())
	if result != nil {
		t.Error("DeriveFromNested(nil) phải trả về nil")
	}
}

func TestDeriveFromNested_EmptyMap_ReturnsNil(t *testing.T) {
	m := map[string]interface{}{}
	result := DeriveFromNested(m, time.Now().UnixMilli())
	if result != nil {
		t.Error("DeriveFromNested(map rỗng không có raw) phải trả về nil")
	}
}

func TestDeriveFromNested_MapWithoutRaw_ReturnsNil(t *testing.T) {
	m := map[string]interface{}{
		"layer1": map[string]interface{}{"orderCount": 1},
	}
	result := DeriveFromNested(m, time.Now().UnixMilli())
	if result != nil {
		t.Error("DeriveFromNested(map không có key 'raw') phải trả về nil")
	}
}

func TestDeriveFromMap_NilMap_ReturnsNil(t *testing.T) {
	result := DeriveFromMap(nil, time.Now().UnixMilli())
	if result != nil {
		t.Error("DeriveFromMap(nil) phải trả về nil")
	}
}

func TestDeriveFromMap_FirstStageCustomer_ReturnsFirstAggregate(t *testing.T) {
	endMs := time.Now().UnixMilli()
	m := map[string]interface{}{
		"journeyStage": "first",
		"orderCount":   1,
		"lifecycleStage": "active",
		"valueTier":    "entry",
		"avgOrderValue": 100000.0,
		"cancelledOrderCount": 0,
		"lastOrderAt":  endMs - 7*24*60*60*1000, // 7 ngày trước
	}
	result := DeriveFromMap(m, endMs)
	if result == nil {
		t.Fatal("DeriveFromMap(first customer) không được trả về nil")
	}
	if result.First == nil {
		t.Error("DeriveFromMap(first customer) phải có First aggregate")
	}
	if result.First != nil {
		if result.First.PurchaseQuality == "" {
			t.Error("First.PurchaseQuality không được rỗng")
		}
		if result.First.ExperienceQuality == "" {
			t.Error("First.ExperienceQuality không được rỗng")
		}
	}
}

func TestDeriveFromMap_EngagedStage_ReturnsEngagedAggregate(t *testing.T) {
	endMs := time.Now().UnixMilli()
	m := map[string]interface{}{
		"journeyStage": "engaged",
		"orderCount":   0,
		"lifecycleStage": "active",
		"totalMessages": 5,
		"lastConversationAt": endMs - 2*24*60*60*1000, // 2 ngày trước
	}
	result := DeriveFromMap(m, endMs)
	if result == nil {
		t.Fatal("DeriveFromMap(engaged customer) không được trả về nil")
	}
	if result.Engaged == nil {
		t.Error("DeriveFromMap(engaged customer) phải có Engaged aggregate")
	}
}

func TestDeriveFromNested_ValidNestedMap_ReturnsAggregate(t *testing.T) {
	endMs := time.Now().UnixMilli()
	m := map[string]interface{}{
		"raw": map[string]interface{}{
			"journeyStage": "first",
			"orderCount":   1,
			"lifecycleStage": "active",
			"valueTier":    "entry",
			"avgOrderValue": 200000.0,
			"cancelledOrderCount": 0,
			"lastOrderAt":  endMs - 10*24*60*60*1000,
		},
		"layer1": map[string]interface{}{},
		"layer2": map[string]interface{}{},
	}
	result := DeriveFromNested(m, endMs)
	if result == nil {
		t.Fatal("DeriveFromNested(nested map hợp lệ) không được trả về nil")
	}
	if result.First == nil {
		t.Error("DeriveFromNested(nested first customer) phải có First aggregate")
	}
}
