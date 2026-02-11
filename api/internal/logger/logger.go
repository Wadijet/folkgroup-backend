package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// loggers map lưu các logger instances
	loggers   = make(map[string]*logrus.Logger)
	loggersMu sync.Mutex

	// config chứa cấu hình logging
	config *LogConfig

	// rootDir lưu đường dẫn gốc của project
	rootDir string
)

// Init khởi tạo hệ thống logging với cấu hình
func Init(cfg *LogConfig) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	config = cfg

	// Lấy rootDir
	if err := initRootDir(); err != nil {
		return fmt.Errorf("failed to initialize root directory: %w", err)
	}

	// Tạo thư mục logs nếu chưa tồn tại
	logPath := getLogPath()
	if err := os.MkdirAll(logPath, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	return nil
}

// initRootDir khởi tạo rootDir của project
func initRootDir() error {
	if rootDir != "" {
		return nil
	}

	// Bước 1: Thử lấy từ environment variable LOG_ROOT_DIR (ưu tiên cao nhất)
	if envRootDir := os.Getenv("LOG_ROOT_DIR"); envRootDir != "" {
		// Resolve symlinks trên Linux
		resolvedPath, err := filepath.EvalSymlinks(envRootDir)
		if err == nil {
			rootDir = resolvedPath
			return nil
		}
		// Nếu không resolve được, dùng đường dẫn gốc
		rootDir = envRootDir
		return nil
	}

	// Bước 2: Thử lấy từ executable path
	executable, err := os.Executable()
	if err == nil {
		// Resolve symlinks trên Linux (quan trọng khi chạy qua systemd)
		resolvedExecutable, err := filepath.EvalSymlinks(executable)
		if err == nil {
			executable = resolvedExecutable
		}

		// Lấy đường dẫn gốc của project (2 cấp trên thư mục cmd)
		// Ví dụ: /path/to/api/cmd/server/main -> /path/to/api
		rootDir = filepath.Dir(filepath.Dir(filepath.Dir(executable)))
		
		// Kiểm tra xem đường dẫn có hợp lệ không (có thư mục logs hoặc config)
		if _, err := os.Stat(filepath.Join(rootDir, "logs")); err == nil {
			return nil
		}
		if _, err := os.Stat(filepath.Join(rootDir, "config")); err == nil {
			return nil
		}
	}

	// Bước 3: Fallback: sử dụng working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get executable or working directory: %v", err)
	}

	// Tìm thư mục api bằng cách đi lên từ working directory
	currentDir := wd
	for i := 0; i < 5; i++ { // Tối đa đi lên 5 cấp
		// Kiểm tra xem có thư mục logs hoặc config không
		if _, err := os.Stat(filepath.Join(currentDir, "logs")); err == nil {
			rootDir = currentDir
			return nil
		}
		if _, err := os.Stat(filepath.Join(currentDir, "config")); err == nil {
			rootDir = currentDir
			return nil
		}

		// Đi lên thư mục cha
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break // Đã đến root
		}
		currentDir = parentDir
	}

	// Nếu không tìm thấy, dùng working directory (2 cấp trên)
	rootDir = filepath.Dir(filepath.Dir(wd))
	return nil
}

// getLogPath trả về đường dẫn thư mục logs
func getLogPath() string {
	if filepath.IsAbs(config.LogPath) {
		return config.LogPath
	}
	return filepath.Join(rootDir, config.LogPath)
}

// GetLogger trả về logger theo tên (app, audit, performance, error)
func GetLogger(name string) *logrus.Logger {
	loggersMu.Lock()
	defer loggersMu.Unlock()

	// Nếu chưa init, init với config mặc định
	if config == nil {
		if err := Init(nil); err != nil {
			panic(fmt.Sprintf("Failed to initialize logger: %v", err))
		}
	}

	// Trả về logger đã tồn tại
	if logger, ok := loggers[name]; ok {
		return logger
	}

	// Tạo logger mới
	logger := createLogger(name)
	loggers[name] = logger

	return logger
}

// createLogger tạo một logger mới với cấu hình
func createLogger(name string) *logrus.Logger {
	logger := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// Set formatter
	if config.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
				logrus.FieldKeyFunc:  "function",
				logrus.FieldKeyFile:  "file",
			},
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05.000",
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, ".")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line)
			},
		})
	}

	// Set output
	// ⚠️ QUAN TRỌNG: Tách file writer và stdout writer để tránh blocking
	// Nếu dùng MultiWriter, khi file I/O chậm sẽ block cả stdout
	// Giải pháp: Dùng async hook cho tất cả writers để tránh blocking request handling

	var writers []io.Writer

	// File output với rotation
	if config.Output == "file" || config.Output == "both" {
		logFile := getLogFilePath(name)
		fileWriter := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    config.MaxSize,    // MB
			MaxBackups: config.MaxBackups, // Số file cũ giữ lại
			MaxAge:     config.MaxAge,     // Số ngày
			Compress:   config.Compress,   // Nén file cũ
		}
		writers = append(writers, fileWriter)
	}

	// Stdout output
	if config.Output == "stdout" || config.Output == "both" {
		writers = append(writers, os.Stdout)
	}

	// Thêm FilterHook trước AsyncHook để filter trước khi ghi log
	// FilterHook phải được thêm trước để filter entries trước khi đưa vào async queue
	filterHook := NewFilterHook(config)
	logger.AddHook(filterHook)

	// Dùng async hook cho tất cả writers để tránh blocking
	// Buffer size: 1000 entries (có thể config sau nếu cần)
	if len(writers) > 0 {
		asyncHook := NewAsyncHookWithWriters(writers, 1000)
		logger.AddHook(asyncHook)
		// Không set output để tránh duplicate logs
		// Hook sẽ xử lý tất cả logging
		logger.SetOutput(io.Discard) // Discard output để chỉ dùng hook
	}

	// Bật caller logging
	logger.SetReportCaller(true)

	// Thêm service name vào mỗi log entry
	logger = logger.WithField("service", name).Logger

	// Log thông tin khởi tạo
	logger.WithFields(logrus.Fields{
		"log_file": getLogFilePath(name),
		"level":    logger.GetLevel().String(),
		"format":   config.Format,
		"output":   config.Output,
	}).Info("Logger initialized successfully")

	return logger
}

// getLogFilePath trả về đường dẫn file log cho logger name
func getLogFilePath(name string) string {
	logPath := getLogPath()
	var filename string

	switch name {
	case "app":
		filename = config.AppFile
	case "audit":
		filename = config.AuditFile
	case "performance":
		filename = config.PerformanceFile
	case "error":
		filename = config.ErrorFile
	default:
		filename = fmt.Sprintf("%s.log", name)
	}

	return filepath.Join(logPath, filename)
}

// GetAppLogger trả về logger chính của ứng dụng
func GetAppLogger() *logrus.Logger {
	return GetLogger("app")
}

// GetAuditLogger trả về logger cho audit
func GetAuditLogger() *logrus.Logger {
	return GetLogger("audit")
}

// GetPerformanceLogger trả về logger cho performance
func GetPerformanceLogger() *logrus.Logger {
	return GetLogger("performance")
}

// GetErrorLogger trả về logger cho errors
func GetErrorLogger() *logrus.Logger {
	return GetLogger("error")
}
