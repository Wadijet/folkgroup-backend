package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

// Configuration chứa thông tin tĩnh cần thiết để chạy ứng dụng
// Nó chứa thông tin cơ sở dữ liệu
type Configuration struct {
	InitMode               bool   `env:"INITMODE" envDefault:"false"`               // Chế độ khởi tạo
	Address                string `env:"ADDRESS" envDefault:":8080"`                // Địa chỉ server
	JwtSecret              string `env:"JWT_SECRET,required"`                       // Bí mật JWT
	MongoDB_ConnectionURI  string `env:"MONGODB_CONNECTION_URI,required"`           // URL kết nối cơ sở dữ liệu
	MongoDB_DBName_Auth    string `env:"MONGODB_DBNAME_AUTH,required"`              // Tên cơ sở dữ liệu xác thực
	MongoDB_DBName_Staging string `env:"MONGODB_DBNAME_STAGING,required"`           // Tên cơ sở dữ liệu staging
	MongoDB_DBName_Data    string `env:"MONGODB_DBNAME_DATA,required"`              // Tên cơ sở dữ liệu data
	CORS_Origins           string `env:"CORS_ORIGINS" envDefault:"*"`               // Các origins được phép (phân cách bởi dấu phẩy, * = tất cả)
	CORS_AllowCredentials  bool   `env:"CORS_ALLOW_CREDENTIALS" envDefault:"false"` // Cho phép gửi credentials
	RateLimit_Max          int    `env:"RATE_LIMIT_MAX" envDefault:"100"`           // Số request tối đa trong window (0 = disable rate limit)
	RateLimit_Window       int    `env:"RATE_LIMIT_WINDOW" envDefault:"60"`         // Thời gian window (giây)
	RateLimit_Enabled      bool   `env:"RATE_LIMIT_ENABLED" envDefault:"true"`      // Bật/tắt rate limiting
	// Firebase Configuration
	FirebaseProjectID       string `env:"FIREBASE_PROJECT_ID"`       // Firebase Project ID
	FirebaseCredentialsPath string `env:"FIREBASE_CREDENTIALS_PATH"` // Đường dẫn đến service account JSON
	FirebaseAPIKey          string `env:"FIREBASE_API_KEY"`          // Firebase Web API Key (cho frontend)
	FirebaseAdminUID        string `env:"FIREBASE_ADMIN_UID"`        // Firebase UID của user admin (tự động tạo admin user trong init)
	// Frontend URL
	FrontendURL string `env:"FRONTEND_URL" envDefault:"http://localhost:3000"` // URL frontend
	// TLS/HTTPS Configuration
	EnableTLS   bool   `env:"ENABLE_TLS" envDefault:"false"` // Bật HTTPS
	TLSCertFile string `env:"TLS_CERT_FILE"`                 // Đường dẫn đến file certificate (.crt hoặc .pem)
	TLSKeyFile  string `env:"TLS_KEY_FILE"`                  // Đường dẫn đến file private key (.key)
	// Telegram Notification Configuration (optional - dùng cho notification init)
	TelegramBotToken    string `env:"TELEGRAM_BOT_TOKEN"`    // Bot token cho Telegram sender mặc định (optional)
	TelegramBotUsername string `env:"TELEGRAM_BOT_USERNAME"` // Bot username cho Telegram sender mặc định (optional)
	TelegramChatIDs     string `env:"TELEGRAM_CHAT_IDS"`     // Danh sách chat IDs phân cách bằng dấu phẩy, ví dụ: "-123456789,-987654321" (optional)
}

// getEnvPath trả về đường dẫn đến file env dựa trên môi trường
// Ưu tiên: ENV_FILE_PATH (đường dẫn tuyệt đối) > ENV_FILE_DIR (thư mục) > Tìm trong cây thư mục
func getEnvPath() string {
	// Mặc định sử dụng môi trường development
	env := os.Getenv("GO_ENV")
	if env == "" {
		env = "development"
	}

	// Bước 1: Kiểm tra ENV_FILE_PATH (đường dẫn tuyệt đối đến file env)
	// Ví dụ: ENV_FILE_PATH=/home/dungdm/folkform/config/production.env
	if envFilePath := os.Getenv("ENV_FILE_PATH"); envFilePath != "" {
		// Kiểm tra file có tồn tại không
		if _, err := os.Stat(envFilePath); err == nil {
			return envFilePath
		}
		// Nếu không tìm thấy, log warning nhưng vẫn tiếp tục tìm các cách khác
		fmt.Printf("[Config] ⚠️  ENV_FILE_PATH được set nhưng file không tồn tại: %s\n", envFilePath)
	}

	// Bước 2: Kiểm tra ENV_FILE_DIR (thư mục chứa file env)
	// Ví dụ: ENV_FILE_DIR=/home/dungdm/folkform/config
	if envFileDir := os.Getenv("ENV_FILE_DIR"); envFileDir != "" {
		// Thử tìm file {GO_ENV}.env (ví dụ: production.env, development.env)
		envPath := filepath.Join(envFileDir, fmt.Sprintf("%s.env", env))
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
		// Thử với tên file backend.env (tên file mặc định trên VPS)
		envPath = filepath.Join(envFileDir, "backend.env")
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
		// Thử với tên file .env (không có prefix environment)
		envPath = filepath.Join(envFileDir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}

	// Bước 3: Tìm trong cây thư mục hiện tại (cho development)
	currentDir, err := os.Getwd()
	if err != nil {
		// Sử dụng fmt.Printf vì logger có thể chưa được init ở đây
		fmt.Printf("Không thể lấy được thư mục hiện tại: %v\n", err)
		return ""
	}

	// Tìm thư mục config/env
	for {
		envDir := filepath.Join(currentDir, "config", "env")
		if _, err := os.Stat(envDir); err == nil {
			// Tìm thấy thư mục config/env
			envPath := filepath.Join(envDir, fmt.Sprintf("%s.env", env))
			return envPath
		}

		// Đi lên thư mục cha
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return ""
		}
		currentDir = parentDir
	}
}

// NewConfig sẽ đọc dữ liệu cấu hình từ environment variables hoặc file env
// Ưu tiên: Environment variables (systemd EnvironmentFile) > File env (development)
func NewConfig(files ...string) *Configuration {
	fmt.Println("[Config] ========================================")
	fmt.Println("[Config] Bắt đầu đọc cấu hình Backend...")

	cfg := Configuration{}

	// Bước 1: Thử load từ file env (cho development, optional)
	fmt.Println("[Config] [Bước 1/2] Kiểm tra file env (development)...")
	// Nếu có systemd EnvironmentFile, env vars sẽ override file
	envPath := getEnvPath()
	if envPath != "" {
		fmt.Printf("[Config] [Bước 1/2] Tìm file env tại: %s\n", envPath)
		// Load file nhưng không fail nếu không tìm thấy
		// File env chỉ dùng cho development, production dùng systemd EnvironmentFile
		if err := godotenv.Load(envPath); err != nil {
			// Chỉ log warning, không fail - sẽ dùng environment variables
			fmt.Printf("[Config] [Bước 1/2] ⚠️  Warning: Không thể load file env tại %s: %v\n", envPath, err)
			fmt.Println("[Config] [Bước 1/2] Sẽ dùng environment variables (systemd EnvironmentFile)")
		} else {
			fmt.Printf("[Config] [Bước 1/2] ✅ Đã load file env từ %s\n", envPath)
		}
	} else {
		fmt.Println("[Config] [Bước 1/2] Không tìm thấy thư mục config/env, bỏ qua file env")
	}

	// Bước 2: Parse từ environment variables (ưu tiên)
	fmt.Println("[Config] [Bước 2/2] Parse từ environment variables (systemd EnvironmentFile)...")
	// env.Parse sẽ đọc từ os.Getenv()
	// Systemd EnvironmentFile sẽ load env vars vào os.Getenv() trước khi chạy
	// Nếu có env vars từ systemd, chúng sẽ override giá trị từ file
	err := env.Parse(&cfg)
	if err != nil {
		fmt.Printf("[Config] [Bước 2/2] ❌ Lỗi khi parse config: %+v\n", err)
		fmt.Println("[Config] ========================================")
		return nil
	}

	fmt.Println("[Config] [Bước 2/2] ✅ Parse config thành công")
	fmt.Printf("[Config] [Bước 2/2] Config values:\n")
	fmt.Printf("[Config]   • ADDRESS: %s\n", cfg.Address)
	fmt.Printf("[Config]   • MONGODB_CONNECTION_URI: %s\n", maskMongoURI(cfg.MongoDB_ConnectionURI))
	fmt.Printf("[Config]   • MONGODB_DBNAME_AUTH: %s\n", cfg.MongoDB_DBName_Auth)
	fmt.Printf("[Config]   • MONGODB_DBNAME_STAGING: %s\n", cfg.MongoDB_DBName_Staging)
	fmt.Printf("[Config]   • MONGODB_DBNAME_DATA: %s\n", cfg.MongoDB_DBName_Data)
	fmt.Printf("[Config]   • CORS_ORIGINS: %s\n", cfg.CORS_Origins)
	fmt.Printf("[Config]   • CORS_ALLOW_CREDENTIALS: %v\n", cfg.CORS_AllowCredentials)
	fmt.Printf("[Config]   • FIREBASE_PROJECT_ID: %s\n", cfg.FirebaseProjectID)
	fmt.Printf("[Config]   • FIREBASE_CREDENTIALS_PATH: %s\n", cfg.FirebaseCredentialsPath)
	fmt.Printf("[Config]   • FRONTEND_URL: %s\n", cfg.FrontendURL)
	fmt.Println("[Config] ========================================")

	return &cfg
}

// Helper function để mask MongoDB URI (ẩn password)
func maskMongoURI(uri string) string {
	// Mask password trong MongoDB URI: mongodb://user:password@host:port/db
	if strings.Contains(uri, "@") {
		parts := strings.Split(uri, "@")
		if len(parts) == 2 {
			userPass := parts[0]
			rest := parts[1]
			if strings.Contains(userPass, ":") {
				userParts := strings.Split(userPass, ":")
				if len(userParts) >= 3 {
					// mongodb://user:password
					return userParts[0] + ":***@" + rest
				}
			}
		}
	}
	return uri
}
