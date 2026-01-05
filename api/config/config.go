package config

import (
	"fmt"
	"os"
	"path/filepath"

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
	TelegramChatIDs     string `env:"TELEGRAM_CHAT_IDS"`    // Danh sách chat IDs phân cách bằng dấu phẩy, ví dụ: "-123456789,-987654321" (optional)
}

// getEnvPath trả về đường dẫn đến file env dựa trên môi trường
func getEnvPath() string {
	// Mặc định sử dụng môi trường development
	env := os.Getenv("GO_ENV")
	if env == "" {
		env = "development"
	}

	// Tìm thư mục config
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
	cfg := Configuration{}
	
	// Bước 1: Thử load từ file env (cho development, optional)
	// Nếu có systemd EnvironmentFile, env vars sẽ override file
	envPath := getEnvPath()
	if envPath != "" {
		// Load file nhưng không fail nếu không tìm thấy
		// File env chỉ dùng cho development, production dùng systemd EnvironmentFile
		if err := godotenv.Load(envPath); err != nil {
			// Chỉ log warning, không fail - sẽ dùng environment variables
			fmt.Printf("Warning: Không thể load file env tại %s: %v (sẽ dùng environment variables)\n", envPath, err)
		}
	}
	
	// Bước 2: Parse từ environment variables (ưu tiên)
	// env.Parse sẽ đọc từ os.Getenv()
	// Systemd EnvironmentFile sẽ load env vars vào os.Getenv() trước khi chạy
	// Nếu có env vars từ systemd, chúng sẽ override giá trị từ file
	err := env.Parse(&cfg)
	if err != nil {
		fmt.Printf("Lỗi khi parse config: %+v\n", err)
		return nil
	}
	
	return &cfg
}
