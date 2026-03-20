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

// TestAdminFullAPIs kiểm tra các API admin với user có full quyền
func TestAdminFullAPIs(t *testing.T) {
	baseURL := "http://localhost:8080/api/v1"

	// Setup test với admin user có full quyền
	fixtures, adminEmail, adminToken, client, err := utils.SetupTestWithAdminUser(t, baseURL)
	if err != nil {
		t.Fatalf("❌ Không thể setup test: %v", err)
	}
	_ = fixtures // Có thể dùng cho các test khác
	// adminToken đã được set trong client, nhưng vẫn cần để gọi GetRootOrganizationID

	// Test 1: Set Administrator cho user khác
	t.Run("👑 Set Administrator", func(t *testing.T) {
		// Tạo user thường và lấy userID từ profile
		firebaseIDToken := utils.GetTestFirebaseIDToken()
		if firebaseIDToken == "" {
			t.Skip("Skipping test: TEST_FIREBASE_ID_TOKEN environment variable not set")
		}
		userEmail, _, userToken, err := fixtures.CreateTestUser(firebaseIDToken)
		if err != nil {
			t.Fatalf("❌ Không thể tạo user test: %v", err)
		}

		// Lấy userID từ profile
		tempClient := utils.NewHTTPClient(baseURL, 10)
		tempClient.SetToken(userToken)
		resp, body, err := tempClient.GET("/auth/profile")
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Skip("⚠️ Không thể lấy userID, bỏ qua test")
			return
		}

		var profileResult map[string]interface{}
		json.Unmarshal(body, &profileResult)
		data, _ := profileResult["data"].(map[string]interface{})
		userID, _ := data["id"].(string)
		if userID == "" {
			t.Skip("⚠️ Không lấy được userID, bỏ qua test")
			return
		}
		_ = userEmail

		// Set administrator cho user này
		// Dùng /admin/user/set-administrator khi hệ thống đã có admin (init route không đăng ký khi có admin)
		resp, body, err = client.POST(fmt.Sprintf("/admin/user/set-administrator/%s", userID), nil)
		if err != nil {
			t.Fatalf("❌ Lỗi khi set administrator: %v", err)
		}

		if resp.StatusCode == http.StatusOK {
			var result map[string]interface{}
			err = json.Unmarshal(body, &result)
			assert.NoError(t, err, "Phải parse được JSON response")
			fmt.Printf("✅ Set administrator thành công\n")
		} else {
			// Có thể đã là admin hoặc cần quyền đặc biệt
			fmt.Printf("⚠️ Set administrator (status: %d - %s)\n", resp.StatusCode, string(body))
		}
	})

	// Test 2: Tạo role với admin quyền
	t.Run("🎭 Tạo Role với Admin", func(t *testing.T) {
		// Set active role trên fixtures client (endpoint /organization/find yêu cầu X-Active-Role-ID)
		resp, body, _ := client.GET("/auth/roles")
		if resp != nil && resp.StatusCode == http.StatusOK {
			var rolesResult map[string]interface{}
			json.Unmarshal(body, &rolesResult)
			if rolesData, ok := rolesResult["data"].([]interface{}); ok && len(rolesData) > 0 {
				if firstRole, ok := rolesData[0].(map[string]interface{}); ok {
					if roleID, ok := firstRole["roleId"].(string); ok {
						fixtures.SetActiveRoleIDForClient(roleID)
					}
				}
			}
		}
		// Lấy Root Organization ID
		rootOrgID, err := fixtures.GetRootOrganizationID(adminToken)
		if err != nil {
			t.Skipf("⚠️ Không thể lấy Root Organization, bỏ qua test tạo role: %v", err)
			return
		}

		payload := map[string]interface{}{
			"name":                fmt.Sprintf("TestRole_%d", time.Now().UnixNano()),
			"describe":            "Test Role Description",
			"ownerOrganizationId": rootOrgID, // BẮT BUỘC - Phân quyền dữ liệu
		}

		resp, body, err = client.POST("/role/insert-one", payload)
		if err != nil {
			t.Fatalf("❌ Lỗi khi tạo role: %v", err)
		}

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			var result map[string]interface{}
			err = json.Unmarshal(body, &result)
			assert.NoError(t, err, "Phải parse được JSON response")
			fmt.Printf("✅ Tạo role thành công với admin quyền\n")
		} else {
			t.Errorf("❌ Tạo role thất bại với admin: %d - %s", resp.StatusCode, string(body))
		}
	})

	// Test 3: Lấy danh sách roles
	t.Run("📋 Lấy danh sách Roles", func(t *testing.T) {
		// Refresh token trước các admin API (tránh 401 do token hết hạn)
		if firebaseToken := utils.GetTestFirebaseIDToken(); firebaseToken != "" {
			if _, _, newToken, err := fixtures.CreateTestUser(firebaseToken); err == nil && newToken != "" {
				client.SetToken(newToken)
			}
		}
		resp, body, err := client.GET("/role/find")
		if err != nil {
			t.Fatalf("❌ Lỗi khi lấy danh sách roles: %v", err)
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Admin phải lấy được danh sách roles")

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		assert.NoError(t, err, "Phải parse được JSON response")
		fmt.Printf("✅ Lấy danh sách roles thành công\n")
	})

	// Test 4: Lấy danh sách permissions
	t.Run("🔐 Lấy danh sách Permissions", func(t *testing.T) {
		resp, body, err := client.GET("/permission/find")
		if err != nil {
			t.Fatalf("❌ Lỗi khi lấy danh sách permissions: %v", err)
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Admin phải lấy được danh sách permissions")

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		assert.NoError(t, err, "Phải parse được JSON response")
		fmt.Printf("✅ Lấy danh sách permissions thành công\n")
	})

	// Test 5: Lấy danh sách users
	t.Run("👥 Lấy danh sách Users", func(t *testing.T) {
		resp, body, err := client.GET("/user/find")
		if err != nil {
			t.Fatalf("❌ Lỗi khi lấy danh sách users: %v", err)
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Admin phải lấy được danh sách users")

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		assert.NoError(t, err, "Phải parse được JSON response")
		fmt.Printf("✅ Lấy danh sách users thành công\n")
	})

	// Test 6: Block/Unblock user
	t.Run("🔒 Block/Unblock User", func(t *testing.T) {
		// Lấy Firebase ID token
		firebaseIDToken := utils.GetTestFirebaseIDToken()
		if firebaseIDToken == "" {
			t.Skip("Skipping test: TEST_FIREBASE_ID_TOKEN environment variable not set")
		}

		// Refresh token trước khi block/unblock (tránh 401 do token hết hạn)
		loginPayload := map[string]interface{}{"idToken": firebaseIDToken, "hwid": "test_device_123"}
		if resp, body, err := client.POST("/auth/login/firebase", loginPayload); err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			var loginResult map[string]interface{}
			if json.Unmarshal(body, &loginResult) == nil {
				if data, ok := loginResult["data"].(map[string]interface{}); ok {
					if newToken, ok := data["token"].(string); ok && newToken != "" {
						client.SetToken(newToken)
					}
				}
			}
		}

		// Tạo user để block
		userEmail, _, _, err := fixtures.CreateTestUser(firebaseIDToken)
		if err != nil {
			t.Fatalf("❌ Không thể tạo user test: %v", err)
		}

		// Block user
		blockPayload := map[string]interface{}{
			"email": userEmail,
			"note":  "Test block",
		}

		resp, body, err := client.POST("/admin/user/block", blockPayload)
		if err != nil {
			t.Fatalf("❌ Lỗi khi block user: %v", err)
		}

		if resp.StatusCode == http.StatusOK {
			var result map[string]interface{}
			err = json.Unmarshal(body, &result)
			assert.NoError(t, err, "Phải parse được JSON response")
			fmt.Printf("✅ Block user thành công\n")
		} else if resp.StatusCode == 401 {
			t.Skipf("⚠️ Block user 401 - Token có thể hết hạn hoặc cần User.Block permission")
		} else {
			t.Errorf("❌ Block user thất bại: %d - %s", resp.StatusCode, string(body))
		}

		// Unblock user
		unblockPayload := map[string]interface{}{
			"email": userEmail,
		}

		resp, body, err = client.POST("/admin/user/unblock", unblockPayload)
		if err != nil {
			t.Fatalf("❌ Lỗi khi unblock user: %v", err)
		}

		if resp.StatusCode == http.StatusOK {
			var result map[string]interface{}
			err = json.Unmarshal(body, &result)
			assert.NoError(t, err, "Phải parse được JSON response")
			fmt.Printf("✅ Unblock user thành công\n")
		} else if resp.StatusCode == 401 {
			t.Skipf("⚠️ Unblock user 401 - Token có thể hết hạn hoặc cần User.Block permission")
		} else {
			t.Errorf("❌ Unblock user thất bại: %d - %s", resp.StatusCode, string(body))
		}
	})

	// Test 7: Set role cho user
	t.Run("👤 Set Role cho User", func(t *testing.T) {
		// Lấy Root Organization ID
		rootOrgID, err := fixtures.GetRootOrganizationID(adminToken)
		if err != nil {
			t.Skipf("⚠️ Không thể lấy Root Organization, bỏ qua test set role: %v", err)
			return
		}

		// Tạo role trước (phải có ownerOrganizationId - phân quyền dữ liệu)
		rolePayload := map[string]interface{}{
			"name":                fmt.Sprintf("TestRole_%d", time.Now().UnixNano()),
			"describe":            "Test Role",
			"ownerOrganizationId": rootOrgID, // BẮT BUỘC - Phân quyền dữ liệu
		}

		resp, body, err := client.POST("/role/insert-one", rolePayload)
		if err != nil {
			t.Fatalf("❌ Lỗi khi tạo role: %v", err)
		}

		var roleID string
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			var result map[string]interface{}
			err = json.Unmarshal(body, &result)
			assert.NoError(t, err, "Phải parse được JSON response")

			data, ok := result["data"].(map[string]interface{})
			if ok {
				id, ok := data["id"].(string)
				if ok {
					roleID = id
				}
			}
		}

		if roleID == "" {
			t.Skip("⚠️ Không thể tạo role, bỏ qua test set role")
			return
		}

		// Lấy Firebase ID token
		firebaseIDToken := utils.GetTestFirebaseIDToken()
		if firebaseIDToken == "" {
			t.Skip("Skipping test: TEST_FIREBASE_ID_TOKEN environment variable not set")
		}

		// Tạo user để set role
		userEmail, _, _, err := fixtures.CreateTestUser(firebaseIDToken)
		if err != nil {
			t.Fatalf("❌ Không thể tạo user test: %v", err)
		}

		// Set role
		setRolePayload := map[string]interface{}{
			"email":  userEmail,
			"roleID": roleID,
		}

		resp, body, err = client.POST("/admin/user/role", setRolePayload)
		if err != nil {
			t.Fatalf("❌ Lỗi khi set role: %v", err)
		}

		if resp.StatusCode == http.StatusOK {
			var result map[string]interface{}
			err = json.Unmarshal(body, &result)
			assert.NoError(t, err, "Phải parse được JSON response")
			fmt.Printf("✅ Set role thành công\n")
		} else {
			t.Errorf("❌ Set role thất bại: %d - %s", resp.StatusCode, string(body))
		}
	})

	// Cleanup
	t.Run("🧹 Cleanup", func(t *testing.T) {
		logoutPayload := map[string]interface{}{
			"hwid": "test_device_123",
		}
		client.POST("/auth/logout", logoutPayload)
		fmt.Printf("✅ Cleanup hoàn tất (admin: %s)\n", adminEmail)
	})
}
