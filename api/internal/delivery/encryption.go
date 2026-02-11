package delivery

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"meta_commerce/internal/global"
)

// getEncryptionKey tạo encryption key từ JWT_SECRET
func getEncryptionKey() []byte {
	secret := global.MongoDB_ServerConfig.JwtSecret
	hash := sha256.Sum256([]byte(secret + "_sender_config_encryption_key"))
	return hash[:]
}

// EncryptSenderConfig mã hóa sender config thành base64 string
func EncryptSenderConfig(configJSON []byte) (string, error) {
	key := getEncryptionKey()
	
	// Tạo AES cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Tạo GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Tạo nonce (12 bytes cho GCM)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, configJSON, nil)

	// Encode to base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSenderConfig giải mã sender config từ base64 string
func DecryptSenderConfig(encryptedBase64 string) ([]byte, error) {
	key := getEncryptionKey()

	// Decode base64
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Tạo AES cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Tạo GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Kiểm tra độ dài
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	// Extract nonce và ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}
