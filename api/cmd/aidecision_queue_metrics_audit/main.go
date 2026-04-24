// Chương trình đối chiếu metrics command center với MongoDB (decision_events_queue).
//
// Chạy từ thư mục api (cùng env với server):
//
//	go run ./cmd/aidecision_queue_metrics_audit
//	go run ./cmd/aidecision_queue_metrics_audit 698c341c977ebc6295312ad8
//
// In ra: đếm theo status (toàn DB hoặc lọc theo ownerOrganizationId), và pipeline giống ReconcileQueueDepthFromMongo.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"meta_commerce/config"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	cfg := config.NewConfig()
	if cfg == nil {
		log.Fatal("Không thể đọc cấu hình (config.NewConfig)")
	}

	orgFilter := ""
	if len(os.Args) > 1 && os.Args[1] != "" {
		orgFilter = os.Args[1]
		if _, err := primitive.ObjectIDFromHex(orgFilter); err != nil {
			log.Fatalf("ownerOrganizationId hex không hợp lệ: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB_ConnectionURI))
	if err != nil {
		log.Fatalf("Kết nối MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(cfg.MongoDB_DBName_Auth)
	coll := db.Collection("decision_job_events")

	activeStatuses := []string{
		aidecisionmodels.EventStatusPending,
		aidecisionmodels.EventStatusLeased,
		aidecisionmodels.EventStatusProcessing,
		aidecisionmodels.EventStatusFailedRetryable,
		aidecisionmodels.EventStatusFailedTerminal,
		aidecisionmodels.EventStatusDeferred,
	}

	fmt.Printf("DB: %s | collection: decision_events_queue\n", cfg.MongoDB_DBName_Auth)
	if orgFilter != "" {
		fmt.Printf("Lọc org: %s\n", orgFilter)
	}
	fmt.Println()

	// 1) Mọi giá trị status trong collection (phát hiện typo / status lạ)
	fmt.Println("=== Đếm theo status (toàn collection, mọi document) ===")
	pAll := mongo.Pipeline{
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$status"},
			{Key: "c", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "c", Value: -1}}}},
	}
	printAgg(ctx, coll, pAll)

	// 2) Bản ghi thiếu ownerOrganizationId (reconcile backend BỎ QUA)
	fmt.Println("\n=== Bản ghi không có ownerOrganizationId (không vào metrics theo org) ===")
	pNoOrg := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "$or", Value: bson.A{
				bson.D{{Key: "ownerOrganizationId", Value: bson.D{{Key: "$exists", Value: false}}}},
				bson.D{{Key: "ownerOrganizationId", Value: nil}},
			}},
		}}},
		bson.D{{Key: "$count", Value: "n"}},
	}
	cur, err := coll.Aggregate(ctx, pNoOrg)
	if err != nil {
		log.Printf("aggregate no-org: %v", err)
	} else {
		defer cur.Close(ctx)
		if cur.Next(ctx) {
			var doc struct {
				N int64 `bson:"n"`
			}
			_ = cur.Decode(&doc)
			fmt.Printf("count: %d\n", doc.N)
		} else {
			fmt.Println("count: 0")
		}
	}

	// 3) Giống ReconcileQueueDepthFromMongo: status ∈ active, group (org, status)
	fmt.Println("\n=== Pipeline reconcile backend (status ∈ active list, group org+status) ===")
	match := bson.D{{Key: "status", Value: bson.D{{Key: "$in", Value: activeStatuses}}}}
	if orgFilter != "" {
		oid, _ := primitive.ObjectIDFromHex(orgFilter)
		match = bson.D{
			{Key: "status", Value: bson.D{{Key: "$in", Value: activeStatuses}}},
			{Key: "ownerOrganizationId", Value: oid},
		}
	}
	pReconcile := mongo.Pipeline{
		bson.D{{Key: "$match", Value: match}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "org", Value: "$ownerOrganizationId"},
				{Key: "st", Value: "$status"},
			}},
			{Key: "c", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "c", Value: -1}}}},
	}
	cur2, err := coll.Aggregate(ctx, pReconcile)
	if err != nil {
		log.Fatalf("aggregate reconcile: %v", err)
	}
	defer cur2.Close(ctx)

	type idDoc struct {
		Org primitive.ObjectID `bson:"org"`
		St  string             `bson:"st"`
	}
	var sumPending, sumLeased, sumProc, sumFailR, sumFailT, sumDef int64
	for cur2.Next(ctx) {
		var row struct {
			ID idDoc `bson:"_id"`
			C  int64 `bson:"c"`
		}
		if err := cur2.Decode(&row); err != nil {
			continue
		}
		if row.ID.Org.IsZero() {
			fmt.Printf("  (org rỗng) status=%q count=%d\n", row.ID.St, row.C)
			continue
		}
		fmt.Printf("  org=%s status=%-20s count=%d\n", row.ID.Org.Hex(), row.ID.St, row.C)
		switch row.ID.St {
		case aidecisionmodels.EventStatusPending:
			sumPending += row.C
		case aidecisionmodels.EventStatusLeased:
			sumLeased += row.C
		case aidecisionmodels.EventStatusProcessing:
			sumProc += row.C
		case aidecisionmodels.EventStatusFailedRetryable:
			sumFailR += row.C
		case aidecisionmodels.EventStatusFailedTerminal:
			sumFailT += row.C
		case aidecisionmodels.EventStatusDeferred:
			sumDef += row.C
		}
	}
	if orgFilter != "" {
		fmt.Printf("\n--- Tổng khớp queueDepth (org đã lọc) ---\n")
		fmt.Printf("pending=%d leased=%d processing=%d failed_retryable=%d failed_terminal=%d deferred=%d in_flight(leased+proc)=%d\n",
			sumPending, sumLeased, sumProc, sumFailR, sumFailT, sumDef, sumLeased+sumProc)
	}
	_ = cur2.Err()
}

func printAgg(ctx context.Context, coll *mongo.Collection, pipeline mongo.Pipeline) {
	cur, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("aggregate: %v", err)
		return
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var row struct {
			ID interface{} `bson:"_id"`
			C  int64       `bson:"c"`
		}
		if err := cur.Decode(&row); err != nil {
			continue
		}
		fmt.Printf("  status=%-30v count=%d\n", row.ID, row.C)
	}
}
