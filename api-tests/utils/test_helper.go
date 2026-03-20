package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// waitForHealth đợi server sẵn sàng trước khi chạy test
func waitForHealth(baseURL string, maxRetries int, retryInterval time.Duration, t *testing.T) {
	client := NewHTTPClient(baseURL, 5)
	
	for i := 0; i < maxRetries; i++ {
		resp, _, err := client.GET("/health")
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			return // Server đã sẵn sàng
		}
		
		if i < maxRetries-1 {
			time.Sleep(retryInterval)
		}
	}
	
	t.Logf("⚠️ Server chưa sẵn sàng sau %d lần thử, tiếp tục test...", maxRetries)
}

// initTestData khởi tạo dữ liệu mặc định của hệ thống
// Bao gồm: Root Organization, Permissions, Roles, Notification Data
func initTestData(t *testing.T, baseURL string) {
	fixtures := NewTestFixtures(baseURL)
	
	// Khởi tạo dữ liệu mặc định (chỉ hoạt động khi chưa có admin)
	err := fixtures.InitData()
	if err != nil {
		// Có thể đã có admin rồi, không phải lỗi
		t.Logf("ℹ️ Init data: %v (có thể đã init rồi)", err)
	}
	
	// Khởi tạo notification data (nếu có endpoint)
	client := NewHTTPClient(baseURL, 10)
	resp, _, _ := client.POST("/init/notification-data", nil)
	if resp != nil && resp.StatusCode == http.StatusOK {
		t.Logf("✅ Notification data đã được khởi tạo")
	} else {
		// Có thể đã init rồi hoặc không có endpoint
		t.Logf("ℹ️ Notification data: có thể đã init rồi hoặc không có endpoint")
	}
}

// SetupTestWithAdminUser setup test với user admin có full quyền
// Trả về: fixtures, adminEmail, adminToken, client
// Lưu ý: Cần có TEST_FIREBASE_ID_TOKEN environment variable
func SetupTestWithAdminUser(t *testing.T, baseURL string) (*TestFixtures, string, string, *HTTPClient, error) {
	// 1. Đợi server sẵn sàng
	waitForHealth(baseURL, 10, 1*time.Second, t)
	
	// 2. Khởi tạo dữ liệu mặc định
	initTestData(t, baseURL)
	
	// 3. Tạo fixtures
	fixtures := NewTestFixtures(baseURL)
	
	// 4. Lấy Firebase ID token
	firebaseIDToken := GetTestFirebaseIDToken()
	if firebaseIDToken == "" {
		return nil, "", "", nil, fmt.Errorf("TEST_FIREBASE_ID_TOKEN environment variable không được set")
	}
	
	// 5. Login với user đã có full quyền admin (không cần CreateAdminUser)
	adminEmail, _, adminToken, err := fixtures.CreateTestUser(firebaseIDToken)
	if err != nil {
		return nil, "", "", nil, fmt.Errorf("không thể login: %v", err)
	}
	
	// 6. Tạo client với admin token
	client := NewHTTPClient(baseURL, 10)
	client.SetToken(adminToken)
	
	// 7. Set active role (nếu có) - cần cho GetRootOrganizationID và các API yêu cầu X-Active-Role-ID
	resp, body, err := client.GET("/auth/roles")
	if err == nil && resp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err == nil {
			if data, ok := result["data"].([]interface{}); ok && len(data) > 0 {
				if firstRole, ok := data[0].(map[string]interface{}); ok {
					if roleID, ok := firstRole["roleId"].(string); ok && roleID != "" {
						client.SetActiveRoleID(roleID)
						fixtures.SetActiveRoleIDForClient(roleID)
						t.Logf("✅ Đã set active role: %s", roleID)
					}
				}
			}
		}
	}
	
	t.Logf("✅ Setup test thành công với admin user: %s", adminEmail)
	return fixtures, adminEmail, adminToken, client, nil
}

// SetupTestWithNewUser setup test với user mới được tạo tự động
// Tự động tạo user mới từ Firebase ID token, đăng nhập và set làm admin (nếu là user đầu tiên)
// Trả về: fixtures, userEmail, userToken, client
// Lưu ý: Cần có Firebase ID token để tạo user
// ⚠️ DEPRECATED: Sử dụng SetupTestWithRegularUser() thay thế (giống nhau)
func SetupTestWithNewUser(t *testing.T, baseURL string) (*TestFixtures, string, string, *HTTPClient, error) {
	// Function này giống với SetupTestWithRegularUser()
	// Giữ lại để backward compatibility
	return SetupTestWithRegularUser(t, baseURL)
}

// SetupTestWithRegularUser setup test với user thường (không có quyền admin)
// Trả về: fixtures, userEmail, userToken, client
func SetupTestWithRegularUser(t *testing.T, baseURL string) (*TestFixtures, string, string, *HTTPClient, error) {
	// 1. Đợi server sẵn sàng
	waitForHealth(baseURL, 10, 1*time.Second, t)
	
	// 2. Khởi tạo dữ liệu mặc định
	initTestData(t, baseURL)
	
	// 3. Tạo fixtures
	fixtures := NewTestFixtures(baseURL)
	
	// 4. Lấy Firebase ID token
	firebaseIDToken := GetTestFirebaseIDToken()
	if firebaseIDToken == "" {
		return nil, "", "", nil, fmt.Errorf("TEST_FIREBASE_ID_TOKEN environment variable không được set")
	}
	
	// 5. Tạo user thường
	userEmail, _, userToken, err := fixtures.CreateTestUser(firebaseIDToken)
	if err != nil {
		return nil, "", "", nil, fmt.Errorf("không thể tạo user: %v", err)
	}
	
	// 6. Tạo client với user token
	client := NewHTTPClient(baseURL, 10)
	client.SetToken(userToken)
	
	// 7. Set active role (nếu có)
	resp, body, err := client.GET("/auth/roles")
	if err == nil && resp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err == nil {
			if data, ok := result["data"].([]interface{}); ok && len(data) > 0 {
				if firstRole, ok := data[0].(map[string]interface{}); ok {
					if roleID, ok := firstRole["roleId"].(string); ok && roleID != "" {
						client.SetActiveRoleID(roleID)
						t.Logf("✅ Đã set active role: %s", roleID)
					}
				}
			}
		}
	}
	
	t.Logf("✅ Setup test thành công với user: %s", userEmail)
	return fixtures, userEmail, userToken, client, nil
}
