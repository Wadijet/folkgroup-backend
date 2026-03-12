// Quick check: conv 109003588125335_25696865006673078 có tồn tại không
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	for _, p := range []string{"api/config/env/development.env", ".env"} {
		if _, err := os.Stat(p); err == nil {
			godotenv.Load(p)
			break
		}
		if _, err := os.Stat(filepath.Join("..", p)); err == nil {
			godotenv.Load(filepath.Join("..", p))
			break
		}
	}
	uri := os.Getenv("MONGODB_CONNECTION_URI")
	if uri == "" {
		uri = os.Getenv("MONGODB_ConnectionURI")
	}
	dbName := os.Getenv("MONGODB_DBNAME_AUTH")
	if dbName == "" {
		dbName = os.Getenv("MONGODB_DBNAME")
	}
	if uri == "" || dbName == "" {
		log.Fatal("Cần env")
	}
	orgID, _ := primitive.ObjectIDFromHex("69a655f0088600c32e62f955")
	threadId := "109003588125335_25696865006673078"

	ctx := context.Background()
	client, _ := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	defer client.Disconnect(ctx)
	coll := client.Database(dbName).Collection("fb_conversations")

	n, _ := coll.CountDocuments(ctx, bson.M{"ownerOrganizationId": orgID, "conversationId": threadId})
	fmt.Printf("Conv conversationId=%s: %d bản ghi\n", threadId, n)

	// Cũng thử customer_id a0134802
	n2, _ := coll.CountDocuments(ctx, bson.M{"ownerOrganizationId": orgID, "customerId": "a0134802-07a5-4eee-a8d8-bc7470a3cbf9"})
	fmt.Printf("Conv customerId=a0134802-...: %d bản ghi\n", n2)
}
