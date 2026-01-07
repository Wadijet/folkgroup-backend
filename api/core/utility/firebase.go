package utility

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

var (
	firebaseApp  *firebase.App
	firebaseAuth *auth.Client
)

// findAPIDir tìm thư mục api (thư mục chứa config/env)
func findAPIDir() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Tìm thư mục api (có chứa config/env)
	for {
		envDir := filepath.Join(currentDir, "config", "env")
		if _, err := os.Stat(envDir); err == nil {
			// Tìm thấy thư mục api
			return currentDir, nil
		}

		// Đi lên thư mục cha
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return "", fmt.Errorf("không tìm thấy thư mục api")
		}
		currentDir = parentDir
	}
}

// InitFirebase khởi tạo Firebase Admin SDK
func InitFirebase(projectID, credentialsPath string) error {
	// Bước 1: Kiểm tra đường dẫn cố định trên VPS (ưu tiên cao nhất)
	// Đường dẫn: /home/dungdm/folkform/config/firebase-service-account.json
	defaultVPSPath := "/home/dungdm/folkform/config/firebase-service-account.json"
	if _, err := os.Stat(defaultVPSPath); err == nil {
		credentialsPath = defaultVPSPath
	} else {
		// Bước 2: Resolve đường dẫn credentials từ thư mục api (nơi có config/)
		// Nếu đường dẫn là absolute, sử dụng trực tiếp
		if filepath.IsAbs(credentialsPath) {
			// Đường dẫn tuyệt đối, sử dụng trực tiếp
			if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
				return fmt.Errorf("firebase credentials file not found: %s", credentialsPath)
			}
		} else {
			// Đường dẫn relative, cần resolve từ thư mục api
			apiDir, err := findAPIDir()
			if err != nil {
				// #region agent log
				logData, _ := json.Marshal(map[string]interface{}{
					"sessionId":    "debug-session",
					"runId":        "post-fix",
					"hypothesisId": "B",
					"location":     "firebase.go:22",
					"message":      "Không tìm thấy thư mục api",
					"data": map[string]interface{}{
						"error": err.Error(),
					},
					"timestamp": time.Now().UnixMilli(),
				})
				if f, err := os.OpenFile("d:\\Crossborder\\ff_be_auth\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
					f.WriteString(string(logData) + "\n")
					f.Close()
				}
				// #endregion
				return fmt.Errorf("không tìm thấy thư mục api: %v", err)
			}

			// Resolve đường dẫn từ thư mục api
			credentialsPath = filepath.Join(apiDir, credentialsPath)
			// #region agent log
			logData, _ := json.Marshal(map[string]interface{}{
				"sessionId":    "debug-session",
				"runId":        "post-fix",
				"hypothesisId": "B",
				"location":     "firebase.go:22",
				"message":      "Resolved credentials path từ thư mục api",
				"data": map[string]interface{}{
					"apiDir":          apiDir,
					"credentialsPath": credentialsPath,
				},
				"timestamp": time.Now().UnixMilli(),
			})
			if f, err := os.OpenFile("d:\\Crossborder\\ff_be_auth\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				f.WriteString(string(logData) + "\n")
				f.Close()
			}
			// #endregion
		}
	}

	// Kiểm tra file credentials tồn tại
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		// Thử đường dẫn cố định trên VPS nếu file không tìm thấy
		defaultVPSPath := "/home/dungdm/folkform/config/firebase-service-account.json"
		if _, err := os.Stat(defaultVPSPath); err == nil {
			credentialsPath = defaultVPSPath
		} else {
			// #region agent log
			logData, _ := json.Marshal(map[string]interface{}{
				"sessionId":    "debug-session",
				"runId":        "post-fix",
				"hypothesisId": "C",
				"location":     "firebase.go:22",
				"message":      "File credentials không tồn tại sau khi resolve",
				"data": map[string]interface{}{
					"credentialsPath": credentialsPath,
					"defaultVPSPath":  defaultVPSPath,
					"error":           err.Error(),
				},
				"timestamp": time.Now().UnixMilli(),
			})
			if f, err := os.OpenFile("d:\\Crossborder\\ff_be_auth\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				f.WriteString(string(logData) + "\n")
				f.Close()
			}
			// #endregion
			return fmt.Errorf("firebase credentials file not found: %s", credentialsPath)
		}
	}

	// Tạo Firebase app
	opt := option.WithCredentialsFile(credentialsPath)
	app, err := firebase.NewApp(context.Background(), &firebase.Config{
		ProjectID: projectID,
	}, opt)

	if err != nil {
		return fmt.Errorf("failed to initialize Firebase app: %v", err)
	}

	firebaseApp = app

	// Tạo Auth client
	authClient, err := app.Auth(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get Firebase Auth client: %v", err)
	}

	firebaseAuth = authClient
	return nil
}

// GetFirebaseAuth trả về Firebase Auth client
func GetFirebaseAuth() *auth.Client {
	return firebaseAuth
}

// VerifyIDToken verify Firebase ID token và trả về user info
func VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	if firebaseAuth == nil {
		return nil, fmt.Errorf("firebase auth not initialized")
	}

	token, err := firebaseAuth.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %v", err)
	}

	return token, nil
}

// GetUserByUID lấy thông tin user từ Firebase bằng UID
func GetUserByUID(ctx context.Context, uid string) (*auth.UserRecord, error) {
	if firebaseAuth == nil {
		return nil, fmt.Errorf("firebase auth not initialized")
	}

	user, err := firebaseAuth.GetUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	return user, nil
}
