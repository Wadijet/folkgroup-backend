package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"ff_be_auth_tests/utils"

	"github.com/stretchr/testify/assert"
)

// TestPancakeAPIs kiểm tra các API Pancake integration
func TestPancakeAPIs(t *testing.T) {
	baseURL := "http://localhost:8080/api/v1"
	waitForHealth(baseURL, 10, 1*time.Second, t)

	// Khởi tạo dữ liệu mặc định trước
	initTestData(t, baseURL)

	fixtures := utils.NewTestFixtures(baseURL)

	// Tạo user với token
	firebaseIDToken := utils.GetTestFirebaseIDToken()
	if firebaseIDToken == "" {
		t.Skip("Skipping test: TEST_FIREBASE_ID_TOKEN environment variable not set")
	}
	_, _, token, err := fixtures.CreateTestUser(firebaseIDToken)
	if err != nil {
		t.Fatalf("❌ Không thể tạo user test: %v", err)
	}

	client := utils.NewHTTPClient(baseURL, 10)
	client.SetToken(token)

	// Test Pancake POS Order APIs (đơn POS; legacy pc_orders đã bỏ)
	t.Run("🥞 Pancake POS Order APIs", func(t *testing.T) {
		t.Run("Lấy danh sách đơn POS", func(t *testing.T) {
			resp, body, err := client.GET("/pancake-pos/order/find")
			if err != nil {
				t.Fatalf("❌ Lỗi khi lấy danh sách đơn POS: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ Lấy danh sách đơn POS thành công\n")
			} else {
				fmt.Printf("⚠️ Lấy danh sách đơn POS yêu cầu quyền (status: %d)\n", resp.StatusCode)
			}
		})

		t.Run("Đếm đơn POS", func(t *testing.T) {
			resp, body, err := client.GET("/pancake-pos/order/count")
			if err != nil {
				t.Fatalf("❌ Lỗi khi đếm đơn POS: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ Đếm đơn POS thành công\n")
			} else {
				fmt.Printf("⚠️ Đếm đơn POS yêu cầu quyền (status: %d)\n", resp.StatusCode)
			}
		})
	})

	// Cleanup
	t.Run("🧹 Cleanup", func(t *testing.T) {
		logoutPayload := map[string]interface{}{
			"hwid": "test_device_123",
		}
		client.POST("/auth/logout", logoutPayload)
		fmt.Printf("✅ Cleanup hoàn tất\n")
	})
}
