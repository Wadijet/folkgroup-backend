package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// LogConfig chứa cấu hình cho hệ thống logging
type LogConfig struct {
	// Log Level: trace, debug, info, warn, error, fatal
	Level string `env:"LOG_LEVEL" envDefault:"info"`

	// Log Format: json, text
	Format string `env:"LOG_FORMAT" envDefault:"text"`

	// Log Output: file, stdout, both
	Output string `env:"LOG_OUTPUT" envDefault:"both"`

	// Log Rotation
	MaxSize    int  `env:"LOG_MAX_SIZE" envDefault:"100"`    // MB
	MaxBackups int  `env:"LOG_MAX_BACKUPS" envDefault:"7"`    // Số file cũ giữ lại
	MaxAge     int  `env:"LOG_MAX_AGE" envDefault:"7"`       // Số ngày giữ lại
	Compress   bool `env:"LOG_COMPRESS" envDefault:"true"`    // Nén file cũ

	// Log Paths
	LogPath         string `env:"LOG_PATH" envDefault:"./logs"`
	AppFile         string `env:"LOG_APP_FILE" envDefault:"app.log"`
	AuditFile       string `env:"LOG_AUDIT_FILE" envDefault:"audit.log"`
	PerformanceFile string `env:"LOG_PERF_FILE" envDefault:"performance.log"`
	ErrorFile       string `env:"LOG_ERROR_FILE" envDefault:"error.log"`
}

// getEnvPath trả về đường dẫn đến file env (tương tự như config.getEnvPath)
func getEnvPath() string {
	// Bước 1: Kiểm tra đường dẫn cố định trên VPS (ưu tiên cao nhất)
	defaultVPSPath := "/home/dungdm/folkform/config/backend.env"
	if _, err := os.Stat(defaultVPSPath); err == nil {
		return defaultVPSPath
	}

	// Bước 2: Kiểm tra ENV_FILE_PATH (đường dẫn tuyệt đối đến file env)
	if envFilePath := os.Getenv("ENV_FILE_PATH"); envFilePath != "" {
		if _, err := os.Stat(envFilePath); err == nil {
			return envFilePath
		}
	}

	// Bước 3: Kiểm tra ENV_FILE_DIR (thư mục chứa file backend.env)
	if envFileDir := os.Getenv("ENV_FILE_DIR"); envFileDir != "" {
		envPath := filepath.Join(envFileDir, "backend.env")
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}

	// Bước 4: Fallback về file env local (cho development)
	// Tìm file api/config/env/development.env
	currentDir, err := os.Getwd()
	if err == nil {
		// Tìm thư mục api/config/env
		for {
			envDir := filepath.Join(currentDir, "config", "env")
			if _, err := os.Stat(envDir); err == nil {
				// Tìm thấy thư mục config/env
				localEnvPath := filepath.Join(envDir, "development.env")
				if _, err := os.Stat(localEnvPath); err == nil {
					return localEnvPath
				}
				break
			}

			// Đi lên thư mục cha
			parentDir := filepath.Dir(currentDir)
			if parentDir == currentDir {
				break
			}
			currentDir = parentDir
		}
	}

	// Không tìm thấy file env
	return ""
}

// DefaultConfig trả về cấu hình mặc định
func DefaultConfig() *LogConfig {
	// Load file env nếu có (tương tự như config.NewConfig)
	// Chỉ load nếu chưa có env vars từ systemd
	envPath := getEnvPath()
	if envPath != "" {
		// Chỉ load nếu các env vars quan trọng chưa được set
		// (có thể đã được set từ systemd EnvironmentFile)
		if os.Getenv("LOG_LEVEL") == "" || os.Getenv("LOG_MAX_AGE") == "" {
			if err := godotenv.Load(envPath); err != nil {
				// Không fail, chỉ log warning nếu không load được
				fmt.Printf("[Logger] ⚠️  Không thể load file env tại %s: %v (sẽ dùng giá trị mặc định)\n", envPath, err)
			} else {
				fmt.Printf("[Logger] ✅ Đã load file env từ %s\n", envPath)
			}
		}
	}

	// Lấy environment
	env := os.Getenv("GO_ENV")
	if env == "" {
		env = "development"
	}

	config := &LogConfig{
		Level:           "info",
		Format:          "text",
		Output:          "both",
		MaxSize:         100,
		MaxBackups:      7,
		MaxAge:          7,
		Compress:        true,
		LogPath:         "./logs",
		AppFile:         "app.log",
		AuditFile:       "audit.log",
		PerformanceFile: "performance.log",
		ErrorFile:       "error.log",
	}

	// Điều chỉnh theo môi trường
	if env == "development" {
		config.Level = "debug"
		config.Format = "text"
	} else {
		config.Level = "info"
		config.Format = "json"
	}

	// Override từ environment variables
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		config.Level = strings.ToLower(level)
	}
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		config.Format = strings.ToLower(format)
	}
	if output := os.Getenv("LOG_OUTPUT"); output != "" {
		config.Output = strings.ToLower(output)
	}

	// Override rotation config từ environment variables
	if maxSizeStr := os.Getenv("LOG_MAX_SIZE"); maxSizeStr != "" {
		if maxSize, err := strconv.Atoi(maxSizeStr); err == nil && maxSize > 0 {
			config.MaxSize = maxSize
		}
	}
	if maxBackupsStr := os.Getenv("LOG_MAX_BACKUPS"); maxBackupsStr != "" {
		if maxBackups, err := strconv.Atoi(maxBackupsStr); err == nil && maxBackups >= 0 {
			config.MaxBackups = maxBackups
		}
	}
	if maxAgeStr := os.Getenv("LOG_MAX_AGE"); maxAgeStr != "" {
		if maxAge, err := strconv.Atoi(maxAgeStr); err == nil && maxAge > 0 {
			config.MaxAge = maxAge
		}
	}
	if compressStr := os.Getenv("LOG_COMPRESS"); compressStr != "" {
		if compress, err := strconv.ParseBool(compressStr); err == nil {
			config.Compress = compress
		}
	}

	// Override log paths từ environment variables
	if logPath := os.Getenv("LOG_PATH"); logPath != "" {
		config.LogPath = logPath
	}
	if appFile := os.Getenv("LOG_APP_FILE"); appFile != "" {
		config.AppFile = appFile
	}
	if auditFile := os.Getenv("LOG_AUDIT_FILE"); auditFile != "" {
		config.AuditFile = auditFile
	}
	if perfFile := os.Getenv("LOG_PERF_FILE"); perfFile != "" {
		config.PerformanceFile = perfFile
	}
	if errorFile := os.Getenv("LOG_ERROR_FILE"); errorFile != "" {
		config.ErrorFile = errorFile
	}

	return config
}
