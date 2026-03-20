package database

import (
	"context"
	"fmt"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureDatabaseAndCollections đảm bảo rằng cơ sở dữ liệu và các collection cần thiết tồn tại.
// Nếu cơ sở dữ liệu không tồn tại, nó sẽ được tạo ra. Nếu các collection không tồn tại, chúng sẽ được tạo ra bằng cách
// chèn một tài liệu dummy và sau đó xóa nó.
//
// Tham số:
// - client: Một đối tượng *mongo.Client kết nối tới MongoDB.
//
// Trả về:
// - error: Lỗi nếu có vấn đề xảy ra trong quá trình kiểm tra hoặc tạo cơ sở dữ liệu và collection.
func EnsureDatabaseAndCollections(client *mongo.Client) error {
	dbName := global.MongoDB_ServerConfig.MongoDB_DBName_Auth

	// Tạo 1 context tổng 30 giây để duyệt tất cả collections
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Kiểm tra database
	dbList, err := client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("failed to list databases: %w", err)
	}

	dbExists := false
	for _, name := range dbList {
		if name == dbName {
			dbExists = true
			break
		}
	}
	if !dbExists {
		logger.GetAppLogger().Infof("Database %s does not exist, will create automatically by creating collections", dbName)
	}

	// Tạo database nếu chưa tồn tại
	db := client.Database(dbName)
	collections := []string{}
	v := reflect.ValueOf(global.MongoDB_ColNames)
	for i := 0; i < v.NumField(); i++ {
		collections = append(collections, v.Field(i).String())
	}

	// Kiểm tra và tạo collections
	collList, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	for _, collectionName := range collections {
		// Kiểm tra collection có tồn tại hay không
		exists := false
		for _, existingColl := range collList {
			if existingColl == collectionName {
				exists = true
				break
			}
		}
		// Tạo collection nếu chưa tồn tại
		if !exists {
			logger.GetAppLogger().Infof("Collection %s chưa tồn tại, tạo mới.", collectionName)
			if err := db.CreateCollection(ctx, collectionName); err != nil {
				return fmt.Errorf("failed to create collection %s: %w", collectionName, err)
			}
		}
	}

	logger.GetAppLogger().Infof("Database and collections are ensured in database: %s", dbName)
	return nil
}

// Hàm parseOrder: Trích xuất thứ tự sắp xếp từ tag (1 hoặc -1)
func parseOrder(tag string) int {
	if strings.Contains(tag, "order:-1") {
		return -1 // Nếu tag chứa "order:-1", trả về -1 (giảm dần)
	}
	return 1 // Mặc định trả về 1 (tăng dần)
}

// Hàm parseIndexTag: Phân tách và phân tích tag index
func parseIndexTag(tag string) []map[string]string {
	parts := strings.Split(tag, ";") // Tách tag theo dấu ';'
	result := []map[string]string{}

	for _, part := range parts {
		subParts := strings.Split(part, ",") // Tách từng cấu hình theo dấu ','
		entry := map[string]string{}
		for _, subPart := range subParts {
			kv := strings.Split(subPart, ":") // Tách thành key và value (nếu có)
			if len(kv) == 2 {
				entry[kv[0]] = kv[1]
			} else {
				entry[kv[0]] = ""
			}
		}
		result = append(result, entry)
	}

	return result // Trả về danh sách các cấu hình index
}

func compareIndex(existingIndex bson.M, keys bson.D, options *options.IndexOptions) bool {
	existingKeys, ok := existingIndex["key"].(bson.M)
	if !ok {
		return false
	}

	// So sánh các khóa
	for _, key := range keys {
		existingValue, exists := existingKeys[key.Key]
		if !exists {
			return false
		}

		// Xử lý cho trường hợp 1 / -1
		newVal, isInt := key.Value.(int)
		if isInt {
			// convert existingValue về int (nếu có thể)
			switch ev := existingValue.(type) {
			case int32:
				if int(ev) != newVal {
					return false
				}
			case int64:
				if int(ev) != newVal {
					return false
				}
			case float64:
				if int(ev) != newVal {
					return false
				}
			default:
				return false
			}
		} else {
			// fallback so sánh kiểu cũ
			if existingValue != key.Value {
				return false
			}
		}
	}

	// So sánh các tùy chọn (unique)
	if unique, ok := existingIndex["unique"].(bool); ok && options.Unique != nil {
		if unique != *options.Unique {
			return false
		}
	} else if options.Unique != nil && *options.Unique {
		// index cũ không unique, index mới lại unique => mismatch
		return false
	}

	// So sánh TTL
	if ttl, ok := existingIndex["expireAfterSeconds"].(int32); ok && options.ExpireAfterSeconds != nil {
		if ttl != *options.ExpireAfterSeconds {
			return false
		}
	}

	return true
}

// checkAndReplaceIndex kiểm tra và thay thế index nếu cần thiết
func checkAndReplaceIndex(
	ctx context.Context,
	collection *mongo.Collection,
	existingIndexes map[string]bson.M,
	indexName string,
	keys bson.D,
	options *options.IndexOptions,
) error {
	// Kiểm tra nếu index đã tồn tại
	if existingIndex, exists := existingIndexes[indexName]; exists {
		// So sánh cấu hình index hiện tại với cấu hình mới
		if compareIndex(existingIndex, keys, options) {
			fmt.Printf("Index %s đã tồn tại và đúng cấu hình, bỏ qua...\n", indexName)
			return nil
		}
		// Xóa index nếu cấu hình không khớp
		if _, err := collection.Indexes().DropOne(ctx, indexName); err != nil {
			return fmt.Errorf("không thể xóa index %s: %w", indexName, err)
		}
		fmt.Printf("Đã xóa index cũ: %s\n", indexName)
	}

	// Tạo index mới
	if _, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    keys,
		Options: options,
	}); err != nil {
		return fmt.Errorf("không thể tạo index %s: %w", indexName, err)
	}
	fmt.Printf("Đã tạo index: %s\n", indexName)
	return nil
}

func CreateIndexes(ctx context.Context, collection *mongo.Collection, model interface{}) error {

	fmt.Printf("Bắt đầu xử lý index cho collection: %s\n", collection.Name())

	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	fmt.Printf("Lấy danh sách indexs hiện có.\n")
	cursor, err := collection.Indexes().List(ctx)
	if err != nil {
		return fmt.Errorf("không thể lấy danh sách index: %w", err)
	}
	defer cursor.Close(ctx)

	existingIndexes := map[string]bson.M{}
	for cursor.Next(ctx) {
		var indexInfo bson.M
		if err := cursor.Decode(&indexInfo); err != nil {
			return fmt.Errorf("không thể giải mã thông tin index: %w", err)
		}
		if name, ok := indexInfo["name"].(string); ok {
			existingIndexes[name] = indexInfo
		}
	}

	compoundGroups := map[string]bson.D{}
	compoundOptions := map[string]*options.IndexOptions{}
	compoundUnique := map[string]bool{}       // Track compound indexes cần unique
	compoundSparse := map[string]bool{}       // Track compound indexes cần sparse
	compoundPartialFilter := map[string]string{} // Track compound indexes cần partial filter (path -> $exists:true)

	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		tag, ok := field.Tag.Lookup("index")
		if !ok {
			continue
		}

		bsonField := field.Tag.Get("bson")
		if bsonField == "" || bsonField == "-" {
			continue
		}
		// Bỏ qua phần ",omitempty" trong bson tag
		if idx := strings.Index(bsonField, ","); idx > 0 {
			bsonField = bsonField[:idx]
		}

		indexConfigs := parseIndexTag(tag)
		for _, config := range indexConfigs {

			if _, ok := config["text"]; ok {
				keys := bson.D{{Key: bsonField, Value: "text"}} // Định nghĩa kiểu text index
				indexName := bsonField + "_text"
				options := options.Index().SetName(indexName)

				if err := checkAndReplaceIndex(ctx, collection, existingIndexes, indexName, keys, options); err != nil {
					return err
				}
			}

			if _, ok := config["single"]; ok {
				order := parseOrder(tag)
				keys := bson.D{{Key: bsonField, Value: order}}
				indexName := bsonField + "_single"
				options := options.Index().SetName(indexName)

				if err := checkAndReplaceIndex(ctx, collection, existingIndexes, indexName, keys, options); err != nil {
					return err
				}
			}

			if _, ok := config["unique"]; ok {
				keys := bson.D{{Key: bsonField, Value: 1}}
				indexName := bsonField + "_unique"
				options := options.Index().SetName(indexName).SetUnique(true)

				// Kiểm tra xem có sparse trong config không
				// Sparse index cho phép nhiều document không có field này
				// Quan trọng cho email/phone vì user có thể không có email/phone khi dùng Firebase
				if _, hasSparse := config["sparse"]; hasSparse {
					options = options.SetSparse(true)
				}

				if err := checkAndReplaceIndex(ctx, collection, existingIndexes, indexName, keys, options); err != nil {
					return err
				}
			}

			if ttlValue, ok := config["ttl"]; ok {
				ttl, err := strconv.Atoi(ttlValue)
				if err != nil {
					return fmt.Errorf("TTL không hợp lệ: %w", err)
				}
				keys := bson.D{{Key: bsonField, Value: 1}}
				indexName := bsonField + "_ttl"
				options := options.Index().SetExpireAfterSeconds(int32(ttl)).SetName(indexName)

				if err := checkAndReplaceIndex(ctx, collection, existingIndexes, indexName, keys, options); err != nil {
					return err
				}
			}

			if groupName, ok := config["compound"]; ok {
				// partial:1 — field dùng cho partialFilterExpression ($exists:true), không thêm vào keys
				if _, hasPartial := config["partial"]; hasPartial {
					compoundPartialFilter[groupName] = bsonField
					if _, exists := compoundOptions[groupName]; !exists {
						compoundOptions[groupName] = options.Index().SetName(groupName)
					}
				} else {
					order := parseOrder(tag)
					compoundGroups[groupName] = append(compoundGroups[groupName], bson.E{Key: bsonField, Value: order})
					if _, exists := compoundOptions[groupName]; !exists {
						compoundOptions[groupName] = options.Index().SetName(groupName)
					}
				}
				// Kiểm tra xem compound index có cần unique không (từ tên group có chứa "_unique")
				if strings.Contains(groupName, "_unique") {
					compoundUnique[groupName] = true
				}
				// Kiểm tra xem compound index có cần sparse không
				if _, hasSparse := config["sparse"]; hasSparse {
					compoundSparse[groupName] = true
				}
			}
		}
	}

	// Tạo compound index
	for groupName, fields := range compoundGroups {
		opts := compoundOptions[groupName]
		// Apply unique và sparse nếu cần
		if compoundUnique[groupName] {
			opts = opts.SetUnique(true)
		}
		if compoundSparse[groupName] {
			opts = opts.SetSparse(true)
		}
		// Apply partial filter nếu có (chỉ index document có field tồn tại)
		if partialPath := compoundPartialFilter[groupName]; partialPath != "" {
			opts = opts.SetPartialFilterExpression(bson.M{partialPath: bson.M{"$exists": true}})
		}
		if err := checkAndReplaceIndex(ctx, collection, existingIndexes, groupName, fields, opts); err != nil {
			return err
		}
	}

	// Cleanup: Xóa các unique index không còn được định nghĩa trong model
	// Tạo map các field có unique index từ model
	fieldsWithUniqueIndex := make(map[string]bool)
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		tag, ok := field.Tag.Lookup("index")
		if !ok {
			continue
		}

		bsonField := field.Tag.Get("bson")
		if bsonField == "" || bsonField == "-" {
			continue
		}

		indexConfigs := parseIndexTag(tag)
		for _, config := range indexConfigs {
			if _, hasUnique := config["unique"]; hasUnique {
				fieldsWithUniqueIndex[bsonField] = true
				break
			}
		}
	}

	// Xóa các unique index không còn được định nghĩa
	for indexName, indexInfo := range existingIndexes {
		// Chỉ xóa index có pattern {field}_unique
		if strings.HasSuffix(indexName, "_unique") {
			// Lấy tên field từ index name (bỏ phần "_unique")
			fieldName := strings.TrimSuffix(indexName, "_unique")

			// Kiểm tra xem field này có unique index trong model không
			if !fieldsWithUniqueIndex[fieldName] {
				// Kiểm tra xem index này có phải là unique index không
				if unique, ok := indexInfo["unique"].(bool); ok && unique {
					fmt.Printf("Phát hiện unique index không còn được định nghĩa: %s, đang xóa...\n", indexName)
					if _, err := collection.Indexes().DropOne(ctx, indexName); err != nil {
						fmt.Printf("Cảnh báo: Không thể xóa index %s: %v\n", indexName, err)
						// Không return error để không chặn việc tạo index khác
					} else {
						fmt.Printf("Đã xóa index không còn được định nghĩa: %s\n", indexName)
					}
				}
			}
		}
	}

	return nil
}

// CreateCrmActivityIndexes tạo index cho crm_activity_history (ActivityBase).
// Dùng snapshot.profile, snapshot.metrics thay metadata.profileSnapshot, metadata.metricsSnapshot.
func CreateCrmActivityIndexes(ctx context.Context, collection *mongo.Collection) error {
	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "unifiedId", Value: 1}}, Options: options.Index().SetName("unifiedId_single")},
		{Keys: bson.D{{Key: "ownerOrganizationId", Value: 1}}, Options: options.Index().SetName("ownerOrganizationId_single")},
		{Keys: bson.D{{Key: "domain", Value: 1}}, Options: options.Index().SetName("domain_single")},
		{Keys: bson.D{{Key: "activityType", Value: 1}}, Options: options.Index().SetName("activityType_single")},
		{Keys: bson.D{{Key: "activityAt", Value: -1}}, Options: options.Index().SetName("activityAt_single")},
		{Keys: bson.D{{Key: "source", Value: 1}}, Options: options.Index().SetName("source_single")},
		{Keys: bson.D{{Key: "createdAt", Value: -1}}, Options: options.Index().SetName("createdAt_single")},
		{Keys: bson.D{{Key: "ownerOrganizationId", Value: 1}, {Key: "unifiedId", Value: 1}, {Key: "activityAt", Value: -1}}, Options: options.Index().SetName("crm_activity_org_unified_at")},
		{Keys: bson.D{{Key: "ownerOrganizationId", Value: 1}, {Key: "activityType", Value: 1}, {Key: "activityAt", Value: -1}}, Options: options.Index().SetName("crm_activity_org_type_at")},
		{Keys: bson.D{{Key: "ownerOrganizationId", Value: 1}, {Key: "source", Value: 1}, {Key: "activityAt", Value: -1}}, Options: options.Index().SetName("crm_activity_org_source_at")},
		{Keys: bson.D{{Key: "ownerOrganizationId", Value: 1}, {Key: "activityAt", Value: -1}}, Options: options.Index().SetName("crm_activity_org_at_report")},
		{Keys: bson.D{{Key: "ownerOrganizationId", Value: 1}, {Key: "activityAt", Value: -1}}, Options: options.Index().SetName("crm_activity_org_at_report_snapshot").SetPartialFilterExpression(bson.M{"snapshot.metrics": bson.M{"$exists": true}})},
		{Keys: bson.D{{Key: "ownerOrganizationId", Value: 1}, {Key: "createdAt", Value: -1}}, Options: options.Index().SetName("crm_activity_org_created")},
	}
	_, err := collection.Indexes().CreateMany(ctx, indexes)
	return err
}

// CreateAdsActivityIndexes tạo index cho ads_activity_history (ActivityBase + AdAccountId, ObjectType, ObjectId).
func CreateAdsActivityIndexes(ctx context.Context, collection *mongo.Collection) error {
	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "activityType", Value: 1}}, Options: options.Index().SetName("activityType_single")},
		{Keys: bson.D{{Key: "adAccountId", Value: 1}}, Options: options.Index().SetName("adAccountId_single")},
		{Keys: bson.D{{Key: "objectType", Value: 1}}, Options: options.Index().SetName("objectType_single")},
		{Keys: bson.D{{Key: "objectId", Value: 1}}, Options: options.Index().SetName("objectId_single")},
		{Keys: bson.D{{Key: "ownerOrganizationId", Value: 1}}, Options: options.Index().SetName("ownerOrganizationId_single")},
		{Keys: bson.D{{Key: "activityAt", Value: -1}}, Options: options.Index().SetName("activityAt_single")},
		{Keys: bson.D{{Key: "ownerOrganizationId", Value: 1}, {Key: "adAccountId", Value: 1}, {Key: "objectType", Value: 1}, {Key: "objectId", Value: 1}, {Key: "activityAt", Value: -1}}, Options: options.Index().SetName("ads_activity_by_entity")},
	}
	_, err := collection.Indexes().CreateMany(ctx, indexes)
	return err
}

// CreateActionPendingIdempotencyIndex tạo index cho payload.idempotencyKey (Vision 08 idempotency lookup).
func CreateActionPendingIdempotencyIndex(ctx context.Context, collection *mongo.Collection) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "payload.idempotencyKey", Value: 1}},
			Options: options.Index().SetName("payload_idempotencyKey_single").SetSparse(true),
		},
		{
			Keys: bson.D{
				{Key: "ownerOrganizationId", Value: 1},
				{Key: "payload.idempotencyKey", Value: 1},
				{Key: "status", Value: 1},
			},
			Options: options.Index().SetName("action_pending_idempotency_lookup").SetPartialFilterExpression(
				bson.M{"payload.idempotencyKey": bson.M{"$exists": true}},
			),
		},
	}
	_, err := collection.Indexes().CreateMany(ctx, indexes)
	return err
}

// CreateDecisionCaseIndexes tạo index cho decision_cases.
func CreateDecisionCaseIndexes(ctx context.Context, collection *mongo.Collection) error {
	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "caseId", Value: 1}}, Options: options.Index().SetName("caseId_unique").SetUnique(true)},
		{Keys: bson.D{{Key: "ownerOrganizationId", Value: 1}, {Key: "domain", Value: 1}, {Key: "createdAt", Value: -1}}, Options: options.Index().SetName("decision_org_domain_created")},
		{Keys: bson.D{{Key: "ownerOrganizationId", Value: 1}, {Key: "targetType", Value: 1}, {Key: "targetId", Value: 1}}, Options: options.Index().SetName("decision_org_target")},
		{Keys: bson.D{{Key: "caseType", Value: 1}}, Options: options.Index().SetName("caseType_single")},
		{Keys: bson.D{{Key: "caseCategory", Value: 1}}, Options: options.Index().SetName("caseCategory_single")},
		{Keys: bson.D{{Key: "goalCode", Value: 1}}, Options: options.Index().SetName("goalCode_single")},
		{Keys: bson.D{{Key: "result", Value: 1}}, Options: options.Index().SetName("result_single")},
		{Keys: bson.D{{Key: "sourceClosedAt", Value: -1}}, Options: options.Index().SetName("sourceClosedAt_single")},
		{Keys: bson.D{{Key: "sourceRef.refType", Value: 1}, {Key: "sourceRef.refId", Value: 1}}, Options: options.Index().SetName("decision_source_ref")},
	}
	_, err := collection.Indexes().CreateMany(ctx, indexes)
	return err
}

// CreateCrmCustomerProfileIndexes tạo index cho profile nested (profile.name, profile.phoneNumbers).
// Gọi sau CreateIndexes cho CrmCustomer.
func CreateCrmCustomerProfileIndexes(ctx context.Context, collection *mongo.Collection) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "ownerOrganizationId", Value: 1},
				{Key: "profile.name", Value: 1},
			},
			Options: options.Index().SetName("crm_customer_org_name"),
		},
		{
			Keys: bson.D{
				{Key: "ownerOrganizationId", Value: 1},
				{Key: "profile.phoneNumbers", Value: 1},
			},
			Options: options.Index().SetName("crm_customer_org_phones"),
		},
	}
	_, err := collection.Indexes().CreateMany(ctx, indexes)
	return err
}
