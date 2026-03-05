// Script chèn dữ liệu mẫu Meta Ads vào MongoDB (campaigns, adsets, ads, insights).
// Các collection này không có API CRUD public, được sync từ Meta API.
// Chạy: go run scripts/insert_meta_sample_data.go
// Chạy từ thư mục gốc project (có api/config/env/development.env).
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const sampleDataDir = "docs-shared/ai-context/folkform/sample-data"

var metaCollections = []struct {
	file string
	col  string
}{
	{"meta_ad_accounts.json", "meta_ad_accounts"},
	{"meta_campaigns.json", "meta_campaigns"},
	{"meta_adsets.json", "meta_adsets"},
	{"meta_ads.json", "meta_ads"},
	{"meta_ad_insights.json", "meta_ad_insights"},
}

func loadEnv() {
	tryPaths := []string{".env", "api/.env", "config/env/development.env", "api/config/env/development.env"}
	cwd, _ := os.Getwd()
	for _, p := range tryPaths {
		full := filepath.Join(cwd, p)
		if _, err := os.Stat(full); err == nil {
			_ = godotenv.Load(full)
			break
		}
		parent := filepath.Dir(cwd)
		if _, err := os.Stat(filepath.Join(parent, p)); err == nil {
			_ = godotenv.Load(filepath.Join(parent, p))
			break
		}
	}
}

// parseOID chuyển map {"$oid": "hex"} thành primitive.ObjectID
func parseOID(v interface{}) (primitive.ObjectID, bool) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return primitive.NilObjectID, false
	}
	s, ok := m["$oid"].(string)
	if !ok {
		return primitive.NilObjectID, false
	}
	oid, err := primitive.ObjectIDFromHex(s)
	if err != nil {
		return primitive.NilObjectID, false
	}
	return oid, true
}

// convertToBSON chuyển JSON doc (có $oid) sang bson.M
func convertToBSON(doc map[string]interface{}) bson.M {
	result := make(bson.M)
	for k, v := range doc {
		if k == "_id" {
			if oid, ok := parseOID(v); ok {
				result["_id"] = oid
			} else {
				result["_id"] = v
			}
			continue
		}
		if k == "ownerOrganizationId" {
			if oid, ok := parseOID(v); ok {
				result["ownerOrganizationId"] = oid
			} else {
				result["ownerOrganizationId"] = v
			}
			continue
		}
		result[k] = v
	}
	return result
}

func main() {
	loadEnv()
	uri := os.Getenv("MONGODB_CONNECTION_URI")
	dbName := os.Getenv("MONGODB_DBNAME_AUTH")
	if uri == "" {
		uri = os.Getenv("MONGODB_ConnectionURI")
	}
	if uri == "" || dbName == "" {
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH trong .env")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối MongoDB lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)

	// Xóa dữ liệu cũ trong meta collections để đảm bảo đúng ~10 mẫu
	for _, item := range metaCollections {
		coll := db.Collection(item.col)
		if _, err := coll.DeleteMany(ctx, bson.M{}); err != nil {
			log.Printf("  [WARN] Xóa %s: %v", item.col, err)
		} else {
			log.Printf("  [CLEAR] %s", item.col)
		}
	}

	success := 0
	for _, item := range metaCollections {
		path := filepath.Join(sampleDataDir, item.file)
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("  [SKIP] %s: không đọc được file %v", item.col, err)
			continue
		}

		var docs []map[string]interface{}
		if err := json.Unmarshal(data, &docs); err != nil {
			log.Printf("  [SKIP] %s: parse JSON lỗi %v", item.col, err)
			continue
		}
		// Giới hạn ~10 mẫu mỗi collection
		if len(docs) > 10 {
			docs = docs[:10]
		}

		coll := db.Collection(item.col)
		inserted := 0
		for _, d := range docs {
			doc := convertToBSON(d)
			_, err := coll.InsertOne(ctx, doc)
			if err != nil {
				// Bỏ qua duplicate (E11000)
				if mongo.IsDuplicateKeyError(err) {
					continue
				}
				log.Printf("  [WARN] %s insert: %v", item.col, err)
				continue
			}
			inserted++
		}
		log.Printf("  [OK] %s: %d/%d documents inserted", item.col, inserted, len(docs))
		success++
	}

	log.Printf("\nHoàn thành: %d collections, mỗi collection ~10 mẫu.", success)
}
