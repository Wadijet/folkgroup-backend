package identity

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Resolver interface để resolve external ID → uid (crm_customers, cio_sessions, ...).
// Đăng ký qua SetDefaultResolver khi khởi động app.
type Resolver interface {
	ResolveToUid(ctx context.Context, externalId string, source string, ownerOrgID primitive.ObjectID) (string, bool)
}

var defaultResolver Resolver

// SetDefaultResolver đăng ký resolver mặc định (gọi từ initsvc).
func SetDefaultResolver(r Resolver) {
	defaultResolver = r
}

// EnrichIdentity4Layers bổ sung uid, sourceIds, links vào doc (map) trước khi InsertOne.
// Chỉ gọi khi ShouldEnrich(collectionName) = true.
// doc phải có _id (ObjectID). Nếu có ownerOrganizationId sẽ dùng để resolve links.
// Nguyên tắc: đã có rồi thì bỏ qua — không ghi đè dữ liệu identity đã tồn tại.
func EnrichIdentity4Layers(ctx context.Context, collectionName string, doc map[string]interface{}, resolver Resolver) error {
	cfg, ok := GetConfig(collectionName)
	if !ok {
		return nil
	}
	if resolver == nil {
		resolver = defaultResolver
	}

	// 1. Lấy _id
	idVal, ok := doc["_id"]
	if !ok || idVal == nil {
		return fmt.Errorf("document thiếu _id, không thể tạo uid")
	}
	oid, err := toObjectID(idVal)
	if err != nil {
		return fmt.Errorf("_id không hợp lệ: %w", err)
	}

	// 2. uid = prefix + _id.Hex() — chỉ set nếu chưa có hoặc chưa đúng format
	expectedUid := utility.UIDFromObjectID(cfg.Prefix, oid)
	shouldSetUid := true
	if existingUid := doc["uid"]; existingUid != nil {
		s := toString(existingUid)
		if s != "" {
			if s == expectedUid || utility.IsUID(s) {
				shouldSetUid = false // Đã có uid đúng format hoặc hợp lệ → bỏ qua
			}
		}
	}
	if shouldSetUid {
		doc["uid"] = expectedUid
	}

	// 3. sourceIds — extract từ paths, chỉ bổ sung source chưa có
	if len(cfg.SourceKeys) > 0 {
		sourceIds := make(map[string]interface{})
		if existing, ok := doc["sourceIds"].(map[string]interface{}); ok {
			for k, v := range existing {
				sourceIds[k] = v
			}
		}
		for _, sk := range cfg.SourceKeys {
			// Đã có source này → bỏ qua, không ghi đè
			if _, has := sourceIds[sk.Source]; has {
				continue
			}
			if v := getMapValueByPath(doc, sk.Path); v != nil {
				if s := toString(v); s != "" {
					sourceIds[sk.Source] = s
				}
			}
		}
		if len(sourceIds) > 0 {
			doc["sourceIds"] = sourceIds
		}
	}

	// 4. links — extract từ paths, resolve nếu có resolver; chỉ bổ sung link chưa có hoặc chưa resolved
	if len(cfg.LinkKeys) > 0 {
		links := make(map[string]interface{})
		if existing, ok := doc["links"].(map[string]interface{}); ok {
			for k, v := range existing {
				links[k] = v
			}
		}
		ownerOrgID := getOwnerOrgID(doc)
		for _, lk := range cfg.LinkKeys {
			// Đã có link và đã resolved → bỏ qua, không ghi đè
			if existingLink := links[lk.Key]; existingLink != nil {
				if linkMap, ok := existingLink.(map[string]interface{}); ok {
					if status, _ := linkMap["status"].(string); status == LinkStatusResolved {
						if uidVal, _ := linkMap["uid"].(string); uidVal != "" {
							continue
						}
					}
				}
			}
			val := getMapValueByPath(doc, lk.Path)
			if val == nil {
				continue
			}
			extId := toString(val)
			if extId == "" {
				continue
			}
			// Đã là uid format → resolved
			if utility.IsUID(extId) {
				links[lk.Key] = map[string]interface{}{
					"uid":          extId,
					"externalRefs": []interface{}{},
					"status":       LinkStatusResolved,
				}
				continue
			}
			// Resolve external id → uid
			if resolver != nil && ownerOrgID != primitive.NilObjectID {
				if resolvedUid, ok := resolver.ResolveToUid(ctx, extId, lk.Source, ownerOrgID); ok && resolvedUid != "" {
					links[lk.Key] = map[string]interface{}{
						"uid":          resolvedUid,
						"externalRefs": []interface{}{map[string]interface{}{"source": lk.Source, "id": extId}},
						"status":       LinkStatusResolved,
					}
					continue
				}
			}
			// Pending
			src := lk.Source
			if src == "" {
				src = "unknown"
			}
			links[lk.Key] = map[string]interface{}{
				"uid":          "",
				"externalRefs": []interface{}{map[string]interface{}{"source": src, "id": extId}},
				"status":       LinkStatusPendingResolution,
			}
		}
		if len(links) > 0 {
			doc["links"] = links
		}
	}
	return nil
}

func toObjectID(v interface{}) (primitive.ObjectID, error) {
	switch x := v.(type) {
	case primitive.ObjectID:
		return x, nil
	case *primitive.ObjectID:
		if x == nil {
			return primitive.NilObjectID, fmt.Errorf("ObjectID nil")
		}
		return *x, nil
	case string:
		return primitive.ObjectIDFromHex(x)
	default:
		return primitive.NilObjectID, fmt.Errorf("kiểu không hỗ trợ: %T", v)
	}
}

func getOwnerOrgID(doc map[string]interface{}) primitive.ObjectID {
	v := getMapValueByPath(doc, "ownerOrganizationId")
	if v == nil {
		return primitive.NilObjectID
	}
	oid, _ := toObjectID(v)
	return oid
}

// getMapValueByPath lấy giá trị từ map theo path dạng "a.b.c" hoặc "a.0.b" (array index).
func getMapValueByPath(m map[string]interface{}, path string) interface{} {
	if m == nil || path == "" {
		return nil
	}
	parts := strings.Split(path, ".")
	cur := interface{}(m)
	for _, p := range parts {
		if cur == nil {
			return nil
		}
		if i, err := strconv.Atoi(p); err == nil {
			// array index
			arr, ok := cur.([]interface{})
			if !ok || i < 0 || i >= len(arr) {
				return nil
			}
			cur = arr[i]
		} else {
			mp, ok := cur.(map[string]interface{})
			if !ok {
				return nil
			}
			cur = mp[p]
		}
	}
	return cur
}

// GetMapValueByPath lấy giá trị từ map theo path (export cho backfill worker).
func GetMapValueByPath(m map[string]interface{}, path string) interface{} {
	return getMapValueByPath(m, path)
}

// ToString chuyển interface{} sang string (export cho backfill worker).
func ToString(v interface{}) string {
	return toString(v)
}

// GetDefaultResolver trả về resolver mặc định (cho backfill worker).
func GetDefaultResolver() Resolver {
	return defaultResolver
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}
