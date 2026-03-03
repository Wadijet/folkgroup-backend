// Package basesvc - Service quản lý MongoDB: danh sách collections, đếm documents, xóa toàn bộ, export.
package basesvc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDbCollectionInfo thông tin collection: tên, số documents, dung lượng, index.
type MongoDbCollectionInfo struct {
	Name            string `json:"name"`            // Tên collection
	DocCount        int64  `json:"docCount"`        // Số lượng documents
	Protected       bool   `json:"protected"`       // true nếu không cho phép xóa toàn bộ
	Size            int64  `json:"size"`            // Tổng dung lượng dữ liệu (bytes)
	StorageSize     int64  `json:"storageSize"`     // Dung lượng trên disk (bytes)
	AvgObjSize      int64  `json:"avgObjSize"`      // Kích thước trung bình mỗi document (bytes)
	IndexCount      int    `json:"indexCount"`      // Số index
	TotalIndexSize  int64  `json:"totalIndexSize"` // Tổng dung lượng các index (bytes)
}

// protectedCollections danh sách collections hệ thống không cho phép xóa toàn bộ (auth, RBAC).
var protectedCollections = map[string]bool{
	"auth_users":                   true,
	"auth_permissions":             true,
	"auth_roles":                   true,
	"auth_role_permissions":        true,
	"auth_user_roles":             true,
	"auth_organizations":           true,
	"auth_organization_config_items": true,
	"auth_organization_shares":     true,
}

// MongoDbManageService service quản lý MongoDB (list, count, delete all, export, import).
type MongoDbManageService struct{}

// NewMongoDbManageService tạo service quản lý MongoDB.
func NewMongoDbManageService() *MongoDbManageService {
	return &MongoDbManageService{}
}

// ListCollectionsWithCount trả về danh sách collections đã đăng ký kèm số documents và thống kê dung lượng.
// Chỉ lấy các collection từ registry (đã đăng ký trong init).
func (s *MongoDbManageService) ListCollectionsWithCount(ctx context.Context) ([]MongoDbCollectionInfo, error) {
	keys := global.RegistryCollections.ListKeys()
	result := make([]MongoDbCollectionInfo, 0, len(keys))

	for _, name := range keys {
		col, exists := global.RegistryCollections.Get(name)
		if !exists {
			continue
		}
		info := MongoDbCollectionInfo{
			Name:      name,
			DocCount:  -1,
			Protected: protectedCollections[name],
		}

		// Lấy thống kê từ collStats (size, storageSize, avgObjSize, indexCount, totalIndexSize)
		db := col.Database()
		var stats bson.M
		err := db.RunCommand(ctx, bson.D{{Key: "collStats", Value: name}}).Decode(&stats)
		if err != nil {
			logger.GetAppLogger().WithError(err).WithField("collection", name).Warn("Không thể lấy collStats")
			// Fallback: chỉ đếm documents
			count, countErr := col.CountDocuments(ctx, bson.M{})
			if countErr == nil {
				info.DocCount = count
			}
		} else {
			if v, ok := stats["count"]; ok {
				if n, ok := v.(int32); ok {
					info.DocCount = int64(n)
				} else if n, ok := v.(int64); ok {
					info.DocCount = n
				} else if n, ok := v.(int); ok {
					info.DocCount = int64(n)
				}
			}
			if v, ok := stats["size"]; ok {
				info.Size = toInt64(v)
			}
			if v, ok := stats["storageSize"]; ok {
				info.StorageSize = toInt64(v)
			}
			if v, ok := stats["avgObjSize"]; ok {
				info.AvgObjSize = toInt64(v)
			}
			if v, ok := stats["nindexes"]; ok {
				info.IndexCount = toInt(v)
			}
			if v, ok := stats["totalIndexSize"]; ok {
				info.TotalIndexSize = toInt64(v)
			}
		}
		result = append(result, info)
	}
	return result, nil
}

// toInt64 chuyển giá trị từ collStats (int32/int64/float64) sang int64.
func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int32:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

// toInt chuyển giá trị từ collStats sang int.
func toInt(v interface{}) int {
	switch n := v.(type) {
	case int32:
		return int(n)
	case int64:
		return int(n)
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}

// ErrCollectionProtected lỗi khi cố xóa collection được bảo vệ.
var ErrCollectionProtected = common.NewError(common.ErrCodeBusinessOperation, "Collection này được bảo vệ, không cho phép xóa toàn bộ", common.StatusForbidden, nil)

// DeleteAllDocuments xóa toàn bộ documents trong collection.
// Không cho phép xóa các collection được bảo vệ (auth_*).
// Trả về số documents đã xóa.
func (s *MongoDbManageService) DeleteAllDocuments(ctx context.Context, collectionName string) (int64, error) {
	if protectedCollections[collectionName] {
		return 0, ErrCollectionProtected
	}
	col, exists := global.RegistryCollections.Get(collectionName)
	if !exists {
		return 0, fmt.Errorf("collection %s không tồn tại trong registry: %w", collectionName, common.ErrNotFound)
	}

	result, err := col.DeleteMany(ctx, bson.M{})
	if err != nil {
		return 0, common.ConvertMongoError(err)
	}

	logger.GetAppLogger().WithFields(map[string]interface{}{
		"collection":   collectionName,
		"deletedCount": result.DeletedCount,
	}).Warn("🗑️ [MONGODB] Đã xóa toàn bộ documents trong collection")

	return result.DeletedCount, nil
}

// ExportCollectionAsJSON export toàn bộ documents của collection dưới dạng JSON array.
// Định dạng tương thích mongoimport (JSON array).
// Lưu ý: Collection lớn có thể gây timeout hoặc dùng nhiều bộ nhớ.
func (s *MongoDbManageService) ExportCollectionAsJSON(ctx context.Context, collectionName string) ([]byte, error) {
	col, exists := global.RegistryCollections.Get(collectionName)
	if !exists {
		return nil, fmt.Errorf("collection %s không tồn tại trong registry: %w", collectionName, common.ErrNotFound)
	}

	cursor, err := col.Find(ctx, bson.M{})
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	var docs []bson.M
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, common.ConvertMongoError(err)
	}

	// Chuyển _id ObjectID sang chuỗi hex để JSON dễ đọc và mongoimport vẫn nhận
	normalized := make([]map[string]interface{}, len(docs))
	for i, d := range docs {
		normalized[i] = convertBSONToJSONFriendly(d)
	}

	return json.Marshal(normalized)
}

// StreamExportToWriter stream documents ra writer theo định dạng NDJSON (một document mỗi dòng).
// Dùng cho export file lớn, không load toàn bộ vào memory.
func (s *MongoDbManageService) StreamExportToWriter(ctx context.Context, collectionName string, w io.Writer) error {
	col, exists := global.RegistryCollections.Get(collectionName)
	if !exists {
		return fmt.Errorf("collection %s không tồn tại trong registry: %w", collectionName, common.ErrNotFound)
	}

	cursor, err := col.Find(ctx, bson.M{})
	if err != nil {
		return common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	encoder := json.NewEncoder(w)
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return common.ConvertMongoError(err)
		}
		normalized := convertBSONToJSONFriendly(doc)
		if err := encoder.Encode(normalized); err != nil {
			return err
		}
	}
	return cursor.Err()
}

// StreamExportFunc trả về hàm callback cho SendStreamWriter (dùng bufio.Writer).
// Dùng cho export streaming qua HTTP.
func (s *MongoDbManageService) StreamExportFunc(ctx context.Context, collectionName string) (func(*bufio.Writer), error) {
	col, exists := global.RegistryCollections.Get(collectionName)
	if !exists {
		return nil, fmt.Errorf("collection %s không tồn tại trong registry: %w", collectionName, common.ErrNotFound)
	}

	return func(bw *bufio.Writer) {
		cursor, err := col.Find(ctx, bson.M{})
		if err != nil {
			logger.GetAppLogger().WithError(err).Error("StreamExport: Find failed")
			return
		}
		defer cursor.Close(ctx)

		encoder := json.NewEncoder(bw)
		for cursor.Next(ctx) {
			var doc bson.M
			if err := cursor.Decode(&doc); err != nil {
				logger.GetAppLogger().WithError(err).Error("StreamExport: Decode failed")
				return
			}
			normalized := convertBSONToJSONFriendly(doc)
			if err := encoder.Encode(normalized); err != nil {
				logger.GetAppLogger().WithError(err).Error("StreamExport: Encode failed")
				return
			}
			bw.Flush() // Flush mỗi doc để stream kịp thời
		}
		if err := cursor.Err(); err != nil {
			logger.GetAppLogger().WithError(err).Error("StreamExport: Cursor error")
		}
	}, nil
}

// ImportCollectionFromReader import documents từ reader (NDJSON: một doc mỗi dòng).
// Dùng cho file lớn, đọc từng dòng không load toàn bộ vào memory.
func (s *MongoDbManageService) ImportCollectionFromNDJSONStream(ctx context.Context, collectionName string, r io.Reader) (int64, error) {
	col, exists := global.RegistryCollections.Get(collectionName)
	if !exists {
		return 0, fmt.Errorf("collection %s không tồn tại trong registry: %w", collectionName, common.ErrNotFound)
	}

	scanner := bufio.NewScanner(r)
	// Tăng buffer cho dòng dài (document lớn)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // Max 1MB mỗi dòng

	var batch []interface{}
	var totalInserted int64
	opts := options.InsertMany().SetOrdered(false)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var doc bson.M
		if err := bson.UnmarshalExtJSON(line, true, &doc); err != nil {
			return totalInserted, fmt.Errorf("dòng không đúng định dạng JSON: %w", err)
		}
		batch = append(batch, doc)

		if len(batch) >= importBatchSize {
			result, err := col.InsertMany(ctx, batch, opts)
			if err != nil {
				return totalInserted, common.ConvertMongoError(err)
			}
			totalInserted += int64(len(result.InsertedIDs))
			batch = batch[:0]
		}
	}
	if err := scanner.Err(); err != nil {
		return totalInserted, err
	}

	// Insert phần còn lại
	if len(batch) > 0 {
		result, err := col.InsertMany(ctx, batch, opts)
		if err != nil {
			return totalInserted, common.ConvertMongoError(err)
		}
		totalInserted += int64(len(result.InsertedIDs))
	}

	logger.GetAppLogger().WithFields(map[string]interface{}{
		"collection":    collectionName,
		"insertedCount": totalInserted,
	}).Info("📥 [MONGODB] Đã import documents (NDJSON stream) vào collection")

	return totalInserted, nil
}

// ImportCollectionFromJSON import documents từ JSON array vào collection.
// Định dạng: JSON array, mỗi document có thể dùng Extended JSON ({"$oid": "hex"} cho ObjectID).
// Trả về số documents đã import. Dùng InsertMany, batch 500 docs/lần để tránh quá tải.
const importBatchSize = 500

func (s *MongoDbManageService) ImportCollectionFromJSON(ctx context.Context, collectionName string, jsonBytes []byte) (int64, error) {
	col, exists := global.RegistryCollections.Get(collectionName)
	if !exists {
		return 0, fmt.Errorf("collection %s không tồn tại trong registry: %w", collectionName, common.ErrNotFound)
	}

	var rawDocs []json.RawMessage
	if err := json.Unmarshal(jsonBytes, &rawDocs); err != nil {
		return 0, fmt.Errorf("JSON không đúng định dạng array: %w", err)
	}

	if len(rawDocs) == 0 {
		return 0, nil
	}

	var docs []interface{}
	for _, raw := range rawDocs {
		var doc bson.M
		if err := bson.UnmarshalExtJSON(raw, true, &doc); err != nil {
			return 0, fmt.Errorf("document không đúng định dạng Extended JSON: %w", err)
		}
		docs = append(docs, doc)
	}

	// Insert theo batch để tránh quá tải
	var totalInserted int64
	opts := options.InsertMany().SetOrdered(false) // Tiếp tục khi có lỗi duplicate
	for i := 0; i < len(docs); i += importBatchSize {
		end := i + importBatchSize
		if end > len(docs) {
			end = len(docs)
		}
		batch := docs[i:end]
		result, err := col.InsertMany(ctx, batch, opts)
		if err != nil {
			return totalInserted, common.ConvertMongoError(err)
		}
		totalInserted += int64(len(result.InsertedIDs))
	}

	logger.GetAppLogger().WithFields(map[string]interface{}{
		"collection":     collectionName,
		"insertedCount":  totalInserted,
	}).Info("📥 [MONGODB] Đã import documents vào collection")

	return totalInserted, nil
}

// convertBSONToJSONFriendly chuyển document BSON sang map thân thiện JSON (ObjectID -> hex string).
func convertBSONToJSONFriendly(d bson.M) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range d {
		result[k] = convertValueToJSONFriendly(v)
	}
	return result
}

// convertValueToJSONFriendly đệ quy chuyển giá trị BSON sang JSON-friendly.
// ObjectID dùng Extended JSON format {"$oid": "hex"} để mongoimport nhận đúng.
func convertValueToJSONFriendly(v interface{}) interface{} {
	switch val := v.(type) {
	case primitive.ObjectID:
		return map[string]string{"$oid": val.Hex()}
	case bson.M:
		return convertBSONToJSONFriendly(val)
	case []interface{}:
		arr := make([]interface{}, len(val))
		for i, item := range val {
			arr[i] = convertValueToJSONFriendly(item)
		}
		return arr
	case []bson.M:
		arr := make([]interface{}, len(val))
		for i, item := range val {
			arr[i] = convertBSONToJSONFriendly(item)
		}
		return arr
	default:
		return v
	}
}
