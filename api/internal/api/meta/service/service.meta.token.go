// Package metasvc - Service quản lý Meta access token (exchange, load, save).
package metasvc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	metaclient "meta_commerce/internal/api/meta/client"
)

// MetaTokenFile cấu trúc file JSON lưu token dài hạn.
type MetaTokenFile struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`   // Giây còn lại khi lưu
	UpdatedAt   int64  `json:"updated_at"`   // Unix timestamp khi lưu
	ExpiresAt   int64  `json:"expires_at"`   // Unix timestamp khi hết hạn (updated_at + expires_in)
}

// LoadMetaToken đọc token từ file. Trả về token rỗng nếu file không tồn tại hoặc lỗi.
// filePath: đường dẫn từ config (vd: config/meta_token.json). Resolve relative từ working dir.
func LoadMetaToken(filePath string) (string, error) {
	if filePath == "" {
		return "", nil
	}
	path := resolveTokenFilePath(filePath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("đọc file token: %w", err)
	}
	var f MetaTokenFile
	if err := json.Unmarshal(data, &f); err != nil {
		return "", fmt.Errorf("parse file token: %w", err)
	}
	if f.AccessToken == "" {
		return "", nil
	}
	// Kiểm tra hết hạn: nếu expires_at đã qua (trừ 1 ngày buffer) thì coi như hết hạn
	now := time.Now().Unix()
	if f.ExpiresAt > 0 && now > f.ExpiresAt-86400 {
		return "", nil // Token sắp hết hạn hoặc đã hết, không dùng
	}
	return f.AccessToken, nil
}

// SaveMetaToken ghi token vào file.
func SaveMetaToken(filePath string, accessToken string, expiresIn int) error {
	if filePath == "" || accessToken == "" {
		return fmt.Errorf("cần filePath và accessToken")
	}
	path := resolveTokenFilePath(filePath)
	now := time.Now().Unix()
	f := MetaTokenFile{
		AccessToken: accessToken,
		ExpiresIn:   expiresIn,
		UpdatedAt:   now,
		ExpiresAt:   now + int64(expiresIn),
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("tạo thư mục: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("ghi file token: %w", err)
	}
	return nil
}

// resolveTokenFilePath resolve đường dẫn file. Nếu relative, resolve từ working dir.
func resolveTokenFilePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	cwd, err := os.Getwd()
	if err != nil {
		return p
	}
	// Nếu chạy từ api/ hoặc api/cmd/server, cwd có thể khác
	return filepath.Join(cwd, p)
}

// GetEffectiveMetaToken trả về token hiệu lực theo thứ tự ưu tiên:
// 1) env META_ACCESS_TOKEN, 2) file (META_TOKEN_FILE nếu có), 3) config MetaAccessToken.
// Dùng khi khởi động worker hoặc sync.
func GetEffectiveMetaToken(envToken, filePath, configToken string) string {
	if envToken != "" {
		return envToken
	}
	if filePath != "" {
		if t, err := LoadMetaToken(filePath); err == nil && t != "" {
			return t
		}
	}
	return configToken
}

// ExchangeAndSaveMetaToken đổi short-lived token sang long-lived, lưu vào file.
// appID, appSecret, shortLivedToken: từ request/config. filePath: từ config.
// Trả về accessToken mới và error.
func ExchangeAndSaveMetaToken(ctx context.Context, appID, appSecret, shortLivedToken, filePath string) (string, error) {
	if appID == "" || appSecret == "" {
		return "", fmt.Errorf("cần META_APP_ID và META_APP_SECRET để đổi token")
	}
	if shortLivedToken == "" {
		return "", fmt.Errorf("cần shortLivedToken (token ngắn hạn từ Meta Login)")
	}
	token, expiresIn, err := metaclient.ExchangeShortForLongLived(ctx, appID, appSecret, shortLivedToken)
	if err != nil {
		return "", err
	}
	if filePath != "" {
		if err := SaveMetaToken(filePath, token, expiresIn); err != nil {
			return token, fmt.Errorf("đổi token thành công nhưng lưu file thất bại: %w", err)
		}
	}
	return token, nil
}
