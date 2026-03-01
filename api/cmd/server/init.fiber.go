package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	agentrouter "meta_commerce/internal/api/agent/router"
	airouter "meta_commerce/internal/api/ai/router"
	authrouter "meta_commerce/internal/api/auth/router"
	contentrouter "meta_commerce/internal/api/content/router"
	crmrouter "meta_commerce/internal/api/crm/router"
	ctarouter "meta_commerce/internal/api/cta/router"
	deliveryrouter "meta_commerce/internal/api/delivery/router"
	fbrouter "meta_commerce/internal/api/fb/router"
	notificationrouter "meta_commerce/internal/api/notification/router"
	pcrouter "meta_commerce/internal/api/pc/router"
	reportrouter "meta_commerce/internal/api/report/router"
	"meta_commerce/internal/api/router"
	webhookrouter "meta_commerce/internal/api/webhook/router"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
)

// InitFiberApp khởi tạo ứng dụng Fiber với các middleware cần thiết
func InitFiberApp() *fiber.App {
	// Khởi tạo app với cấu hình nâng cao
	app := fiber.New(fiber.Config{
		// =========================================
		// 1. CẤU HÌNH CƠ BẢN
		// =========================================
		AppName:       "Meta Commerce API", // Tên ứng dụng hiển thị
		ServerHeader:  "Meta Commerce API", // Header server trong response
		StrictRouting: true,                // /foo và /foo/ là khác nhau
		CaseSensitive: true,                // /Foo và /foo là khác nhau
		UnescapePath:  true,                // Tự động decode URL-encoded paths
		Immutable:     false,               // Tính năng immutable cho ctx

		// =========================================
		// 2. CẤU HÌNH PERFORMANCE
		// =========================================
		BodyLimit:       10 * 1024 * 1024, // Max size của request body (10MB)
		Concurrency:     256 * 1024,       // Số lượng goroutines tối đa
		ReadBufferSize:  4096,             // Buffer size cho request reading
		WriteBufferSize: 4096,             // Buffer size cho response writing

		// =========================================
		// 3. CẤU HÌNH TIMEOUT
		// =========================================
		ReadTimeout:  15 * time.Second,  // Timeout đọc request
		WriteTimeout: 30 * time.Second,  // Timeout ghi response
		IdleTimeout:  120 * time.Second, // Timeout cho idle connections

		// =========================================
		// 4. CẤU HÌNH ERROR HANDLING
		// =========================================
		ErrorHandler: func(c fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			message := "Internal Server Error"
			errorCode := common.ErrCodeInternalServer.Code

			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
				message = e.Message
				// Map HTTP status code to error code
				switch code {
				case fiber.StatusBadRequest:
					errorCode = common.ErrCodeValidationInput.Code
				case fiber.StatusUnauthorized:
					errorCode = common.ErrCodeAuthToken.Code
				case fiber.StatusForbidden:
					errorCode = common.ErrCodeAuthRole.Code
				case fiber.StatusNotFound:
					errorCode = common.ErrCodeDatabaseQuery.Code
				case fiber.StatusConflict:
					errorCode = common.ErrCodeDatabaseQuery.Code
				}
			}

			// Kiểm tra xem có phải lỗi TLS handshake không (HTTPS đến HTTP server)
			// TLS handshake bắt đầu với byte 0x16 0x03 0x01 (trong error message có thể hiển thị là \x16\x03\x01)
			errMsg := err.Error()
			isTLSHandshake := strings.Contains(errMsg, "unsupported http request method") &&
				(strings.Contains(errMsg, "\\x16\\x03\\x01") ||
					strings.Contains(errMsg, "\x16\x03\x01") ||
					strings.Contains(errMsg, "error when reading request headers"))

			// Nếu là TLS handshake, downgrade log level và trả về lỗi phù hợp
			if isTLSHandshake {
				// Không log TLS handshake để giảm log (đây là hành vi bình thường)

				// Trả về lỗi Bad Request với message hướng dẫn rõ ràng
				c.Set("Content-Type", "application/json; charset=utf-8")
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"code":    common.ErrCodeValidationInput.Code,
					"message": "Server chỉ hỗ trợ HTTP. Vui lòng sử dụng http:// thay vì https://",
					"status":  "error",
					"details": fiber.Map{
						"protocol":   "HTTP only",
						"suggestion": "Sử dụng URL: http://localhost:" + global.MongoDB_ServerConfig.Address,
					},
				})
			}

			// Log error cho các lỗi khác
			logger.WithRequest(c).WithFields(map[string]interface{}{
				"code":      code,
				"errorCode": errorCode,
				"message":   message,
			}).Error("Request error")

			// Return JSON error với format thống nhất
			c.Set("Content-Type", "application/json; charset=utf-8")
			return c.Status(code).JSON(fiber.Map{
				"code":    errorCode,
				"message": message,
				"status":  "error",
			})
		},
	})

	// =========================================
	// MIDDLEWARE STACK
	// =========================================

	// 1. Request ID Middleware - Tạo ID duy nhất cho mỗi request để trace
	app.Use(requestid.New(requestid.Config{
		Header: "X-Request-ID",
		Generator: func() string {
			return fmt.Sprintf("%d", time.Now().UnixNano())
		},
	}))

	// 2. Debug Middleware - Đã tắt để giảm log
	// Chỉ log khi có lỗi hoặc trong development mode
	// app.Use(func(c fiber.Ctx) error {
	// 	return c.Next()
	// })

	// 3. CORS Middleware - PHẢI ĐẶT Ở ĐẦU để xử lý preflight requests trước các middleware khác
	corsOrigins := global.MongoDB_ServerConfig.CORS_Origins
	var allowOrigins []string
	if corsOrigins == "*" {
		// Development mode: cho phép tất cả
		allowOrigins = []string{"*"}
	} else {
		// Production mode: chỉ cho phép các origins cụ thể
		allowOrigins = strings.Split(corsOrigins, ",")
		// Trim spaces
		for i, origin := range allowOrigins {
			allowOrigins[i] = strings.TrimSpace(origin)
		}
	}

	app.Use(cors.New(cors.Config{
		AllowOrigins: allowOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Request-ID",
			"X-Requested-With",
			"X-Active-Role-ID", // Header cho role context (quan trọng)
		},
		AllowCredentials: global.MongoDB_ServerConfig.CORS_AllowCredentials,
		ExposeHeaders:    []string{"Content-Length", "Content-Range", "X-Request-ID"},
		MaxAge:           24 * 60 * 60, // Thời gian cache preflight requests (24 giờ)
		// Fiber v3 tự động trả về 204 No Content cho OPTIONS requests
	}))

	// 4. Security Headers Middleware - Thêm các security headers
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Chỉ set HSTS nếu dùng HTTPS
		// c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		return c.Next()
	})

	// 4.5. JSON Content-Type Middleware - Đảm bảo charset=utf-8 cho JSON responses
	// Middleware này sẽ tự động set Content-Type: application/json; charset=utf-8
	// cho tất cả JSON responses (fallback cho các trường hợp không dùng helper function)
	app.Use(func(c fiber.Ctx) error {
		// Chạy handler trước
		err := c.Next()

		// Sau khi handler chạy xong, kiểm tra và set Content-Type nếu cần
		contentType := c.GetRespHeader("Content-Type")

		// Nếu Content-Type là application/json nhưng chưa có charset, thêm charset=utf-8
		if contentType == "application/json" || (strings.HasPrefix(contentType, "application/json") && !strings.Contains(contentType, "charset")) {
			c.Set("Content-Type", "application/json; charset=utf-8")
		}

		return err
	})

	// 5. Rate Limiting Middleware - Giới hạn số request
	// Chỉ bật rate limit nếu được enable và Max > 0
	if global.MongoDB_ServerConfig.RateLimit_Enabled && global.MongoDB_ServerConfig.RateLimit_Max > 0 {
		rateLimitMax := global.MongoDB_ServerConfig.RateLimit_Max
		rateLimitWindow := time.Duration(global.MongoDB_ServerConfig.RateLimit_Window) * time.Second
		app.Use(limiter.New(limiter.Config{
			Max:        rateLimitMax,
			Expiration: rateLimitWindow,
			KeyGenerator: func(c fiber.Ctx) string {
				return c.IP() // Giới hạn theo IP
			},
			LimitReached: func(c fiber.Ctx) error {
				c.Set("Content-Type", "application/json; charset=utf-8")
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
					"code":    common.ErrCodeBusinessOperation.Code,
					"message": "Quá nhiều yêu cầu, vui lòng thử lại sau",
					"status":  "error",
				})
			},
			SkipFailedRequests:     false,
			SkipSuccessfulRequests: false,
			Next: func(c fiber.Ctx) bool {
				// Bỏ qua rate limit cho health check và OPTIONS requests (preflight)
				return c.Path() == "/health" ||
					c.Path() == "/api/v1/system/health" ||
					c.Method() == "OPTIONS"
			},
		}))
		log := logger.GetAppLogger()
		log.Infof("Rate limiting enabled: %d requests per %d seconds", rateLimitMax, global.MongoDB_ServerConfig.RateLimit_Window)
	} else {
		log := logger.GetAppLogger()
		log.Info("Rate limiting disabled")
	}

	// 6. Recover Middleware
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, e interface{}) {
			// Log panic với stack trace
			logger.WithRequest(c).WithFields(map[string]interface{}{
				"panic":   e,
				"headers": c.GetReqHeaders(),
				"body":    string(c.Body()),
			}).Error("Panic recovered")

			// Trả về response với format chuẩn
			c.Set("Content-Type", "application/json; charset=utf-8")
			c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"code":    fiber.StatusInternalServerError,
				"message": "Internal Server Error",
				"error":   fmt.Sprintf("%v", e),
				"time":    time.Now().Format(time.RFC3339),
			})
		},
		Next: func(c fiber.Ctx) bool {
			// Bỏ qua health check và một số endpoint không cần thiết
			return c.Path() == "/health" ||
				c.Path() == "/metrics" ||
				c.Path() == "/api/v1/system/health"
		},
	}))

	// 7. Logger Middleware - Đã tắt để giảm log (chỉ log errors qua error handler)
	// Fiber logger middleware log mỗi request, tắt để giảm tải
	// app.Use(fiberlogger.New(fiberlogger.Config{
	// 	Format:     "${time} | ${ip} | ${status} | ${latency} | ${method} | ${path} | ${requestID} | ${error}\n",
	// 	TimeFormat: "2006-01-02 15:04:05",
	// 	TimeZone:   "Asia/Ho_Chi_Minh",
	// 	Output:     os.Stdout,
	// 	Next: func(c fiber.Ctx) bool {
	// 		return c.Path() == "/health" || c.Path() == "/api/v1/system/health"
	// 	},
	// }))

	// Khởi tạo routes trước khi đăng ký response middleware (theo từng domain để tránh import cycle)
	if err := router.SetupRoutes(app,
		authrouter.Register,
		fbrouter.Register,
		pcrouter.Register,
		webhookrouter.Register,
		reportrouter.Register,
		crmrouter.Register,
		notificationrouter.Register,
		ctarouter.Register,
		deliveryrouter.Register,
		agentrouter.Register,
		contentrouter.Register,
		airouter.Register,
	); err != nil {
		panic(fmt.Sprintf("setup routes: %v", err))
	}

	return app
}
