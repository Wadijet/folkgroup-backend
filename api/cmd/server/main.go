package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v3"

	"meta_commerce/internal/delivery"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker"
)

// initLogger khởi tạo và cấu hình logger cho toàn bộ ứng dụng
func initLogger() {
	// Khởi tạo logger với cấu hình mặc định
	// Logger sẽ tự động đọc environment variables để cấu hình
	if err := logger.Init(nil); err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	// Log thông tin khởi tạo bằng logger mới
	log := logger.GetAppLogger()
	log.Info("Logger system initialized successfully")
}

// main_thread khởi tạo và chạy Fiber server
func main_thread() {
	// Khởi tạo app với cấu hình
	app := InitFiberApp()

	// Khởi động server với cấu hình listen
	cfg := global.MongoDB_ServerConfig
	address := ":" + cfg.Address
	
	log := logger.GetAppLogger()
	log.Info("Starting Fiber server...")
	
	// Helper function để resolve đường dẫn từ thư mục api
	resolvePath := func(path string) string {
		if filepath.IsAbs(path) {
			return path
		}
		// Tìm thư mục api
		currentDir, err := os.Getwd()
		if err != nil {
			return path
		}
		for {
			envDir := filepath.Join(currentDir, "config", "env")
			if _, err := os.Stat(envDir); err == nil {
				return filepath.Join(currentDir, path)
			}
			parentDir := filepath.Dir(currentDir)
			if parentDir == currentDir {
				return path
			}
			currentDir = parentDir
		}
	}

	// Kiểm tra xem có bật TLS không
	if cfg.EnableTLS && cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		// Resolve đường dẫn certificate và key từ thư mục api
		certPath := resolvePath(cfg.TLSCertFile)
		keyPath := resolvePath(cfg.TLSKeyFile)
		
		// Kiểm tra file certificate và key tồn tại
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			log.Fatalf("TLS certificate file not found: %s (resolved from: %s)", certPath, cfg.TLSCertFile)
		}
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			log.Fatalf("TLS key file not found: %s (resolved from: %s)", keyPath, cfg.TLSKeyFile)
		}
		
		// Load certificate và key
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			log.Fatalf("Error loading TLS certificate: %v", err)
		}
		
		// Tạo listener với TLS
		ln, err := net.Listen("tcp", address)
		if err != nil {
			log.Fatalf("Error creating listener: %v", err)
		}
		
		// Cấu hình TLS
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		
		// Wrap listener với TLS
		tlsListener := tls.NewListener(ln, tlsConfig)
		
		log.WithFields(map[string]interface{}{
			"address": address,
			"cert":    certPath,
			"key":     keyPath,
		}).Info("Starting server with HTTPS/TLS")
		
		// Khởi động server với TLS listener
		if err := app.Listener(tlsListener); err != nil {
			log.Fatalf("Error in Fiber Listener with TLS: %v", err)
		}
	} else {
		// Khởi động server HTTP thông thường
		log.WithFields(map[string]interface{}{
			"address":  address,
			"protocol": "HTTP",
		}).Info("Starting server with HTTP")
		
		listenConfig := fiber.ListenConfig{}
		if err := app.Listen(address, listenConfig); err != nil {
			log.Fatalf("Error in Fiber Listen: %v", err)
		}
	}
}

// Hàm main
func main() {
	// Khởi tạo logger
	initLogger()

	// Khởi tạo các biến toàn cục
	InitGlobal()

	// Khởi tạo registry
	InitRegistry()

	// Khởi tạo dữ liệu mặc định
	InitDefaultData()

	// Khởi tạo và chạy Delivery Processor (background worker - Hệ thống 1)
	// Lấy base URL từ environment variable hoặc dùng default
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		// Default base URL nếu không có config
		cfg := global.MongoDB_ServerConfig
		protocol := "http"
		if cfg.EnableTLS {
			protocol = "https"
		}
		baseURL = fmt.Sprintf("%s://localhost:%s", protocol, cfg.Address)
	}
	
	log := logger.GetAppLogger()
	processor, err := delivery.NewProcessor(baseURL)
	if err != nil {
		log.WithError(err).Error("Failed to create delivery processor, continuing without delivery worker")
	} else {
		// Tạo context với cancel để có thể dừng processor khi cần
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Chạy processor trong goroutine riêng với recover
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{
						"panic": r,
					}).Error("📦 [DELIVERY] Processor goroutine panic, processor sẽ tự khởi động lại")
				}
			}()
			
			log.Info("📦 [DELIVERY] Starting Delivery Processor...")
			processor.Start(ctx)
			log.Warn("📦 [DELIVERY] Processor đã dừng (có thể do context cancelled)")
		}()

		log.Info("📦 [DELIVERY] Delivery Processor started successfully")
	}

	// Khởi tạo và chạy Command Cleanup Worker (background worker - Module 2)
	// Worker này tự động giải phóng các AI workflow commands bị stuck
	commandCleanupWorker, err := worker.NewCommandCleanupWorker(1*time.Minute, 300) // Chạy mỗi 1 phút, timeout 5 phút
	if err != nil {
		log.WithError(err).Error("Failed to create command cleanup worker, continuing without cleanup worker")
	} else {
		// Tạo context với cancel để có thể dừng worker khi cần
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Chạy worker trong goroutine riêng với recover
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{
						"panic": r,
					}).Error("🔄 [COMMAND_CLEANUP] Worker goroutine panic, worker sẽ tự khởi động lại")
				}
			}()

			log.Info("🔄 [COMMAND_CLEANUP] Starting Command Cleanup Worker...")
			commandCleanupWorker.Start(ctx)
			log.Warn("🔄 [COMMAND_CLEANUP] Worker đã dừng (có thể do context cancelled)")
		}()

		log.Info("🔄 [COMMAND_CLEANUP] Command Cleanup Worker started successfully")
	}

	// Khởi tạo và chạy Agent Command Cleanup Worker (background worker - Agent Management)
	// Worker này tự động giải phóng các agent commands bị stuck
	agentCommandCleanupWorker, err := worker.NewAgentCommandCleanupWorker(1*time.Minute, 300) // Chạy mỗi 1 phút, timeout 5 phút
	if err != nil {
		log.WithError(err).Error("Failed to create agent command cleanup worker, continuing without cleanup worker")
	} else {
		// Tạo context với cancel để có thể dừng worker khi cần
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Chạy worker trong goroutine riêng với recover
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{
						"panic": r,
					}).Error("🔄 [AGENT_COMMAND_CLEANUP] Worker goroutine panic, worker sẽ tự khởi động lại")
				}
			}()

			log.Info("🔄 [AGENT_COMMAND_CLEANUP] Starting Agent Command Cleanup Worker...")
			agentCommandCleanupWorker.Start(ctx)
			log.Warn("🔄 [AGENT_COMMAND_CLEANUP] Worker đã dừng (có thể do context cancelled)")
		}()

		log.Info("🔄 [AGENT_COMMAND_CLEANUP] Agent Command Cleanup Worker started successfully")
	}

	// Worker báo cáo theo chu kỳ: xử lý report_dirty_periods (Compute → set processedAt)
	reportDirtyWorker, err := worker.NewReportDirtyWorker(1*time.Minute, 50)
	if err != nil {
		log.WithError(err).Warn("Failed to create report dirty worker, continuing without report worker")
	} else {
		ctxReport, cancelReport := context.WithCancel(context.Background())
		defer cancelReport()

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{
						"panic": r,
					}).Error("📊 [REPORT_DIRTY] Worker goroutine panic")
				}
			}()
			log.Info("📊 [REPORT_DIRTY] Starting Report Dirty Worker...")
			reportDirtyWorker.Start(ctxReport)
			log.Warn("📊 [REPORT_DIRTY] Worker đã dừng")
		}()

		log.Info("📊 [REPORT_DIRTY] Report Dirty Worker started successfully")
	}

	// Worker tính lại phân loại khách hàng (full: hàng ngày; smart: mỗi 6h, chỉ khách gần ngưỡng)
	classificationRefreshFullWorker, err := worker.NewClassificationRefreshWorker(24*time.Hour, 200, worker.ClassificationRefreshModeFull)
	if err != nil {
		log.WithError(err).Warn("Failed to create classification refresh full worker")
	} else {
		ctxClassFull, cancelClassFull := context.WithCancel(context.Background())
		defer cancelClassFull()
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{"panic": r}).Error("📊 [CLASSIFICATION_FULL] Worker panic")
				}
			}()
			log.Info("📊 [CLASSIFICATION_FULL] Starting Classification Refresh Worker (full mode)...")
			classificationRefreshFullWorker.Start(ctxClassFull)
		}()
		log.Info("📊 [CLASSIFICATION_FULL] Classification Refresh Full Worker started (chạy mỗi 24h)")
	}

	classificationRefreshSmartWorker, err := worker.NewClassificationRefreshWorker(6*time.Hour, 200, worker.ClassificationRefreshModeSmart)
	if err != nil {
		log.WithError(err).Warn("Failed to create classification refresh smart worker")
	} else {
		ctxClassSmart, cancelClassSmart := context.WithCancel(context.Background())
		defer cancelClassSmart()
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{"panic": r}).Error("📊 [CLASSIFICATION_SMART] Worker panic")
				}
			}()
			log.Info("📊 [CLASSIFICATION_SMART] Starting Classification Refresh Worker (smart mode)...")
			classificationRefreshSmartWorker.Start(ctxClassSmart)
		}()
		log.Info("📊 [CLASSIFICATION_SMART] Classification Refresh Smart Worker started (chạy mỗi 6h)")
	}

	// Chạy Fiber server trên main thread
	main_thread()
}
