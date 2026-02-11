package database

import (
	"context"
	"fmt"
	"meta_commerce/config"
	"meta_commerce/internal/logger"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetInstance initializes and returns a *mongo.Client object.
// This function uses the database connection URL from the provided configuration.
//
// Parameters:
// - c: Pointer to the config.Configuration object containing configuration information.
//
// Returns:
// - *mongo.Client: The connected MongoDB client object.
//
// Notes:
// - This function will log and return an error if there is an issue during connection or connection check.
func GetInstance(c *config.Configuration) (*mongo.Client, error) {
	if c.MongoDB_ConnectionURI == "" {
		return nil, fmt.Errorf("database connection URL is empty")
	}

	// Cài đặt các options cho client
	clientOptions := options.Client().ApplyURI(c.MongoDB_ConnectionURI).
		SetMaxPoolSize(50).                 // Giới hạn tối đa 50 connections
		SetMinPoolSize(10).                 // Giữ tối thiểu 10 connections trong pool
		SetConnectTimeout(5 * time.Second). // Timeout khi kết nối
		SetSocketTimeout(10 * time.Second)  // Timeout khi gửi nhận dữ liệu

	// Kết nối thử với MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Tạo client
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Kiểm tra kết nối
	ctxPing, cancelPing := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelPing()

	err = client.Ping(ctxPing, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	logger.GetAppLogger().Info("Successfully connected to MongoDB")
	return client, nil
}

// CloseInstance closes the MongoDB client connection.
func CloseInstance(client *mongo.Client) error {
	if err := client.Disconnect(context.TODO()); err != nil {
		logger.GetAppLogger().WithError(err).Error("Failed to disconnect MongoDB client")
		return err
	}
	logger.GetAppLogger().Info("Successfully disconnected from MongoDB")
	return nil
}
