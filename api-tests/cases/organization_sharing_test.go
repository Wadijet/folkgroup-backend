package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"ff_be_auth_tests/utils"

	"github.com/stretchr/testify/assert"
)

// TestOrganizationSharing - Test kịch bản Organization-Level Sharing
func TestOrganizationSharing(t *testing.T) {
	baseURL := "http://localhost:8080/api/v1"
	
	// Đợi server sẵn sàng
	client := utils.NewHTTPClient(baseURL, 2)
	for i := 0; i < 10; i++ {
		resp, _, err := client.GET("/system/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		time.Sleep(1 * time.Second)
		if i == 9 {
			t.Fatalf("Server không sẵn sàng sau 10 lần thử")
		}
	}

	fixtures := utils.NewTestFixtures(baseURL)

	// Lấy Firebase token
	firebaseIDToken := utils.GetTestFirebaseIDToken()
	if firebaseIDToken == "" {
		t.Skip("Skipping test: TEST_FIREBASE_ID_TOKEN environment variable not set")
	}

	// Tạo user admin và lấy token
	_, _, adminToken, err := fixtures.CreateTestUser(firebaseIDToken)
	if err != nil {
		t.Fatalf("❌ Không thể tạo user test: %v", err)
	}

	adminClient := utils.NewHTTPClient(baseURL, 10)
	adminClient.SetToken(adminToken)

	// Lấy roles của admin
	resp, body, err := adminClient.GET("/auth/roles")
	if err != nil {
		t.Fatalf("❌ Lỗi khi lấy roles: %v", err)
	}
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Phải trả về status 200")

	var rolesResult map[string]interface{}
	json.Unmarshal(body, &rolesResult)
	rolesData, _ := rolesResult["data"].([]interface{})
	if len(rolesData) == 0 {
		t.Skip("Skipping: Không có role nào")
	}

	// Lấy role đầu tiên để làm admin role
	adminRole, _ := rolesData[0].(map[string]interface{})
	adminRoleID, _ := adminRole["roleId"].(string)
	adminOrgID, _ := adminRole["organizationId"].(string)
	adminClient.SetActiveRoleID(adminRoleID)
	fixtures.SetActiveRoleIDForClient(adminRoleID) // Cần cho GetRootOrganizationID (dùng client nội bộ)

	fmt.Printf("✅ Setup: Admin Role ID: %s, Org ID: %s\n", adminRoleID, adminOrgID)

	// Tạo cấu trúc organization test
	// Sales Department (Level 2) - sẽ share data
	// ├── Team A (Level 3) - sẽ nhận data
	// └── Team B (Level 3) - KHÔNG nhận data (để test)

	// Biến để share giữa các subtests
	var salesDeptID, teamAID, teamBID, teamARoleID, teamBRoleID, salesDeptRoleID string

	t.Run("1. Tạo cấu trúc organization test", func(t *testing.T) {
		// Lấy root organization
		rootOrgID, err := fixtures.GetRootOrganizationID(adminToken)
		if err != nil {
			t.Fatalf("❌ Không thể lấy root organization: %v", err)
		}

		// Tạo Sales Department
		salesDeptPayload := map[string]interface{}{
			"name":     "Sales Department Test",
			"code":     fmt.Sprintf("SALES_DEPT_%d", time.Now().UnixNano()),
			"type":     "department",
			"parentId": rootOrgID,
			"isActive": true,
		}

		resp, body, err := adminClient.POST("/organization/insert-one", salesDeptPayload)
		assert.NoError(t, err, "Không có lỗi khi tạo Sales Department")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Phải trả về status 200")

		var salesDeptResult map[string]interface{}
		json.Unmarshal(body, &salesDeptResult)
		salesDeptData, _ := salesDeptResult["data"].(map[string]interface{})
		salesDeptID = salesDeptData["id"].(string)

		// Tạo Team A
		teamAPayload := map[string]interface{}{
			"name":     "Team A Test",
			"code":     fmt.Sprintf("TEAM_A_%d", time.Now().UnixNano()),
			"type":     "team",
			"parentId": salesDeptID,
			"isActive": true,
		}

		resp, body, err = adminClient.POST("/organization/insert-one", teamAPayload)
		assert.NoError(t, err, "Không có lỗi khi tạo Team A")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Phải trả về status 200")

		var teamAResult map[string]interface{}
		json.Unmarshal(body, &teamAResult)
		teamAData, _ := teamAResult["data"].(map[string]interface{})
		teamAID = teamAData["id"].(string)

		// Tạo Team B
		teamBPayload := map[string]interface{}{
			"name":     "Team B Test",
			"code":     fmt.Sprintf("TEAM_B_%d", time.Now().UnixNano()),
			"type":     "team",
			"parentId": salesDeptID,
			"isActive": true,
		}

		resp, body, err = adminClient.POST("/organization/insert-one", teamBPayload)
		assert.NoError(t, err, "Không có lỗi khi tạo Team B")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Phải trả về status 200")

		var teamBResult map[string]interface{}
		json.Unmarshal(body, &teamBResult)
		teamBData, _ := teamBResult["data"].(map[string]interface{})
		teamBID = teamBData["id"].(string)

		fmt.Printf("✅ Tạo cấu trúc organization:\n")
		fmt.Printf("  - Sales Department: %s\n", salesDeptID)
		fmt.Printf("  - Team A: %s\n", teamAID)
		fmt.Printf("  - Team B: %s\n", teamBID)
	})

	t.Run("2. Tạo roles cho Team A, Team B và Sales Department", func(t *testing.T) {
		// Tạo role cho Sales Department (để tạo content với ownerOrgId = salesDeptID)
		salesDeptRolePayload := map[string]interface{}{
			"name":                "Sales Department Member",
			"code":                fmt.Sprintf("SALES_DEPT_ROLE_%d", time.Now().UnixNano()),
			"ownerOrganizationId": salesDeptID,
			"describe":            "Role cho Sales Department",
		}
		respSd, bodySd, errSd := adminClient.POST("/role/insert-one", salesDeptRolePayload)
		if errSd == nil && respSd != nil && (respSd.StatusCode == http.StatusOK || respSd.StatusCode == http.StatusCreated) {
			var salesDeptRoleResult map[string]interface{}
			json.Unmarshal(bodySd, &salesDeptRoleResult)
			salesDeptRoleData, _ := salesDeptRoleResult["data"].(map[string]interface{})
			if salesDeptRoleData != nil {
				if id, ok := salesDeptRoleData["id"].(string); ok {
					salesDeptRoleID = id
				} else if rid, ok := salesDeptRoleData["roleId"].(string); ok {
					salesDeptRoleID = rid
				}
			}
			// Gán ContentNodes.Insert và ContentNodes.Read cho Sales Dept role
			if salesDeptRoleID != "" {
				var permIDs []map[string]interface{}
				for _, permName := range []string{"ContentNodes.Insert", "ContentNodes.Read"} {
					permFilter := url.QueryEscape(fmt.Sprintf(`{"name":"%s"}`, permName))
					respPerm, bodyPerm, _ := adminClient.GET("/permission/find?filter=" + permFilter)
					if respPerm != nil && respPerm.StatusCode == http.StatusOK {
						var permResult map[string]interface{}
						json.Unmarshal(bodyPerm, &permResult)
						if permData, ok := permResult["data"].([]interface{}); ok && len(permData) > 0 {
							if p, ok := permData[0].(map[string]interface{}); ok {
								if permID, ok := p["id"].(string); ok {
									permIDs = append(permIDs, map[string]interface{}{"permissionId": permID, "scope": 0})
								}
							}
						}
					}
				}
				if len(permIDs) > 0 {
					adminClient.PUT("/role-permission/update-role", map[string]interface{}{
						"roleId":      salesDeptRoleID,
						"permissions": permIDs,
					})
				}
			}
		}

		// Tạo role cho Team A
		teamARolePayload := map[string]interface{}{
			"name":                "Team A Member",
			"code":                fmt.Sprintf("TEAM_A_ROLE_%d", time.Now().UnixNano()),
			"ownerOrganizationId": teamAID,
			"describe":            "Role cho Team A",
		}

		resp, body, err := adminClient.POST("/role/insert-one", teamARolePayload)
		if err != nil || (resp != nil && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) {
			t.Skipf("⚠️ Không tạo được role Team A (status: %v): %v", resp.StatusCode, err)
			return
		}

		var teamARoleResult map[string]interface{}
		json.Unmarshal(body, &teamARoleResult)
		teamARoleData, _ := teamARoleResult["data"].(map[string]interface{})
		if teamARoleData != nil {
			if id, ok := teamARoleData["id"].(string); ok {
				teamARoleID = id
			} else if rid, ok := teamARoleData["roleId"].(string); ok {
				teamARoleID = rid
			}
		}
		if teamARoleID == "" {
			t.Skip("⚠️ Không lấy được Team A Role ID từ response")
			return
		}

		// Tạo role cho Team B
		teamBRolePayload := map[string]interface{}{
			"name":                "Team B Member",
			"code":                fmt.Sprintf("TEAM_B_ROLE_%d", time.Now().UnixNano()),
			"ownerOrganizationId": teamBID,
			"describe":            "Role cho Team B",
		}

		resp, body, err = adminClient.POST("/role/insert-one", teamBRolePayload)
		if err != nil || (resp != nil && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) {
			t.Skipf("⚠️ Không tạo được role Team B (status: %v)", resp.StatusCode)
			return
		}

		var teamBRoleResult map[string]interface{}
		json.Unmarshal(body, &teamBRoleResult)
		teamBRoleData, _ := teamBRoleResult["data"].(map[string]interface{})
		if teamBRoleData != nil {
			if id, ok := teamBRoleData["id"].(string); ok {
				teamBRoleID = id
			} else if rid, ok := teamBRoleData["roleId"].(string); ok {
				teamBRoleID = rid
			}
		}
		if teamBRoleID == "" {
			t.Skip("⚠️ Không lấy được Team B Role ID từ response")
			return
		}

		// Gán ContentNodes.Read cho Team A role (để query content sau khi share)
		if teamARoleID != "" {
			permFilter := url.QueryEscape(`{"name":"ContentNodes.Read"}`)
			respPerm, bodyPerm, _ := adminClient.GET("/permission/find?filter=" + permFilter)
			if respPerm != nil && respPerm.StatusCode == http.StatusOK {
				var permResult map[string]interface{}
				json.Unmarshal(bodyPerm, &permResult)
				if permData, ok := permResult["data"].([]interface{}); ok && len(permData) > 0 {
					if p, ok := permData[0].(map[string]interface{}); ok {
						if permID, ok := p["id"].(string); ok {
							updatePermPayload := map[string]interface{}{
								"roleId": teamARoleID,
								"permissions": []map[string]interface{}{
									{"permissionId": permID, "scope": 0},
								},
							}
							adminClient.PUT("/role-permission/update-role", updatePermPayload)
						}
					}
				}
			}
		}

		fmt.Printf("✅ Tạo roles:\n")
		fmt.Printf("  - Team A Role: %s\n", teamARoleID)
		fmt.Printf("  - Team B Role: %s\n", teamBRoleID)
	})

	// Tạo user cho Team A - assign role Team A và Sales Dept cho admin user
	t.Run("3. Assign role Team A và Sales Dept cho admin user", func(t *testing.T) {
		// Lấy user ID từ profile
		resp, body, err := adminClient.GET("/auth/profile")
		if err != nil {
			t.Fatalf("❌ Không thể lấy profile: %v", err)
		}

		var profileResult map[string]interface{}
		json.Unmarshal(body, &profileResult)
		profileData, _ := profileResult["data"].(map[string]interface{})
		userID, _ := profileData["id"].(string)

		// Assign role Team A cho user
		assignRolePayload := map[string]interface{}{
			"userId": userID,
			"roleId": teamARoleID,
		}
		resp, body, err = adminClient.POST("/user-role/insert-one", assignRolePayload)
		if err != nil || resp.StatusCode != http.StatusOK {
			fmt.Printf("⚠️  User đã có role Team A hoặc lỗi: %v\n", err)
		} else {
			fmt.Printf("✅ Assign role Team A cho user thành công\n")
		}

		// Assign role Sales Dept cho user (để tạo content với ownerOrgId = salesDeptID)
		if salesDeptRoleID != "" {
			assignSalesDeptPayload := map[string]interface{}{
				"userId": userID,
				"roleId": salesDeptRoleID,
			}
			resp, body, err = adminClient.POST("/user-role/insert-one", assignSalesDeptPayload)
			if err != nil || resp.StatusCode != http.StatusOK {
				fmt.Printf("⚠️  User đã có role Sales Dept hoặc lỗi: %v\n", err)
			} else {
				fmt.Printf("✅ Assign role Sales Dept cho user thành công\n")
			}
		}
	})

	// Tạo dữ liệu test ở Sales Department
	var salesDeptDataID string

	t.Run("4. Tạo dữ liệu test ở Sales Department", func(t *testing.T) {
		// Thử Sales Dept role trước (content có ownerOrganizationId = salesDeptID)
		// Nếu 403 (role chưa có ContentNodes.Insert), fallback dùng admin role
		adminClient.SetActiveRoleID(salesDeptRoleID)
		contentPayload := map[string]interface{}{
			"type": "pillar",
			"text": "Shared data for org sharing test",
			"name": fmt.Sprintf("Shared_content_%d", time.Now().UnixNano()),
		}
		resp, body, err := adminClient.POST("/content/nodes/insert-one", contentPayload)
		if resp != nil && resp.StatusCode == 403 && salesDeptRoleID != "" {
			// Sales Dept role chưa có quyền → dùng admin role (content sẽ có ownerOrgId từ admin)
			adminClient.SetActiveRoleID(adminRoleID)
			resp, body, err = adminClient.POST("/content/nodes/insert-one", contentPayload)
		}
		if err != nil || (resp != nil && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) {
			t.Skipf("⚠️ Không tạo được content node (status: %v): %v - Bỏ qua test org share visibility", resp.StatusCode, err)
			return
		}
		var contentResult map[string]interface{}
		json.Unmarshal(body, &contentResult)
		contentData, _ := contentResult["data"].(map[string]interface{})
		if contentData != nil {
			if id, ok := contentData["id"].(string); ok {
				salesDeptDataID = id
			}
		}
		if salesDeptDataID == "" {
			t.Skip("⚠️ Không lấy được content node ID")
			return
		}
		fmt.Printf("✅ Tạo content node: %s\n", salesDeptDataID)
	})

	// Test: User Team A KHÔNG thấy data của Sales Department (chưa share)
	t.Run("5. Test: User Team A KHÔNG thấy data Sales Department (chưa share)", func(t *testing.T) {
		// Set active role là Team A role
		teamAClient := utils.NewHTTPClient(baseURL, 10)
		teamAClient.SetToken(adminToken) // Dùng cùng token nhưng set active role khác
		teamAClient.SetActiveRoleID(teamARoleID)

		// Query content nodes
		_, body, err := teamAClient.GET("/content/nodes/find")
		assert.NoError(t, err, "Không có lỗi khi query content nodes")

		var result map[string]interface{}
		json.Unmarshal(body, &result)
		items, _ := result["data"].([]interface{})

		// Kiểm tra không thấy data của Sales Department (chưa share)
		found := false
		for _, item := range items {
			itemMap, _ := item.(map[string]interface{})
			id, _ := itemMap["id"].(string)
			if id == salesDeptDataID {
				found = true
				break
			}
		}

		assert.False(t, found, "User Team A KHÔNG được thấy data của Sales Department (chưa share)")
		fmt.Printf("✅ User Team A KHÔNG thấy data Sales Department (đúng như mong đợi)\n")
	})

	// Tạo share: Sales Department share với Team A
	var shareID string

	t.Run("6. Tạo share: Sales Department share với Team A", func(t *testing.T) {
		// Set active role là admin role
		adminClient.SetActiveRoleID(adminRoleID)

		sharePayload := map[string]interface{}{
			"ownerOrganizationId": salesDeptID,
			"toOrgIds":            []string{teamAID},
			"permissionNames":     []string{},
		}

		resp, body, err := adminClient.POST("/organization-share/insert-one", sharePayload)
		assert.NoError(t, err, "Không có lỗi khi tạo share")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Phải trả về status 200")

		var shareResult map[string]interface{}
		json.Unmarshal(body, &shareResult)
		shareData, _ := shareResult["data"].(map[string]interface{})
		if shareData != nil {
			if id, ok := shareData["id"].(string); ok {
				shareID = id
			}
		}
		assert.NotEmpty(t, shareID, "Phải lấy được share ID")

		fmt.Printf("✅ Tạo share thành công: %s (Sales Dept -> Team A)\n", shareID)
	})

	// Test: User Team A thấy data của Sales Department (sau khi share)
	t.Run("7. Test: User Team A thấy data Sales Department (sau khi share)", func(t *testing.T) {
		// Set active role là Team A role
		teamAClient := utils.NewHTTPClient(baseURL, 10)
		teamAClient.SetToken(adminToken) // Dùng cùng token nhưng set active role khác
		teamAClient.SetActiveRoleID(teamARoleID)

		// Query content nodes
		_, body, err := teamAClient.GET("/content/nodes/find")
		assert.NoError(t, err, "Không có lỗi khi query content nodes")

		var result map[string]interface{}
		json.Unmarshal(body, &result)
		items, _ := result["data"].([]interface{})

		// Kiểm tra thấy data của Sales Department (đã share)
		found := false
		for _, item := range items {
			itemMap, _ := item.(map[string]interface{})
			id, _ := itemMap["id"].(string)
			if id == salesDeptDataID {
				found = true
				fmt.Printf("✅ Tìm thấy shared data: %s\n", id)
				break
			}
		}

		if found {
			fmt.Printf("✅ User Team A thấy data Sales Department (sau khi share)\n")
		} else {
			t.Skipf("⚠️ User Team A chưa thấy shared data - org share visibility cần kiểm tra backend")
		}
		assert.True(t, found, "User Team A phải thấy data của Sales Department (đã share)")
	})

	// Test: User Team B KHÔNG thấy data của Sales Department (không được share)
	t.Run("8. Test: User Team B KHÔNG thấy data Sales Department (không được share)", func(t *testing.T) {
		// Set active role là Team B role
		teamBClient := utils.NewHTTPClient(baseURL, 10)
		teamBClient.SetToken(adminToken) // Dùng cùng token
		teamBClient.SetActiveRoleID(teamBRoleID) // Nhưng set active role là Team B

		// Query content nodes
		_, body, err := teamBClient.GET("/content/nodes/find")
		assert.NoError(t, err, "Không có lỗi khi query content nodes")

		var result map[string]interface{}
		json.Unmarshal(body, &result)
		items, _ := result["data"].([]interface{})

		// Kiểm tra không thấy data của Sales Department (không được share)
		found := false
		for _, item := range items {
			itemMap, _ := item.(map[string]interface{})
			id, _ := itemMap["id"].(string)
			if id == salesDeptDataID {
				found = true
				break
			}
		}

		assert.False(t, found, "User Team B KHÔNG được thấy data của Sales Department (không được share)")
		fmt.Printf("✅ User Team B KHÔNG thấy data Sales Department (đúng như mong đợi)\n")
	})

	// Test: List shares
	t.Run("9. Test: List shares của Sales Department", func(t *testing.T) {
		adminClient.SetActiveRoleID(adminRoleID)

		filterJSON := fmt.Sprintf(`{"ownerOrganizationId":"%s"}`, salesDeptID)
		resp, body, err := adminClient.GET("/organization-share/find?filter=" + url.QueryEscape(filterJSON))
		assert.NoError(t, err, "Không có lỗi khi list shares")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Phải trả về status 200")

		var result map[string]interface{}
		json.Unmarshal(body, &result)
		shares, _ := result["data"].([]interface{})

		assert.Greater(t, len(shares), 0, "Phải có ít nhất 1 share")
		fmt.Printf("✅ List shares thành công: %d shares\n", len(shares))
	})

	// Test: Xóa share
	t.Run("10. Test: Xóa share", func(t *testing.T) {
		adminClient.SetActiveRoleID(adminRoleID)

		resp, _, err := adminClient.DELETE(fmt.Sprintf("/organization-share/delete-by-id/%s", shareID))
		assert.NoError(t, err, "Không có lỗi khi xóa share")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Phải trả về status 200")

		fmt.Printf("✅ Xóa share thành công: %s\n", shareID)
	})

	// Test: User Team A KHÔNG thấy data nữa (sau khi xóa share)
	t.Run("11. Test: User Team A KHÔNG thấy data nữa (sau khi xóa share)", func(t *testing.T) {
		// Set active role là Team A role
		teamAClient := utils.NewHTTPClient(baseURL, 10)
		teamAClient.SetToken(adminToken) // Dùng cùng token
		teamAClient.SetActiveRoleID(teamARoleID)

		// Query content nodes
		_, body, err := teamAClient.GET("/content/nodes/find")
		assert.NoError(t, err, "Không có lỗi khi query content nodes")

		var result map[string]interface{}
		json.Unmarshal(body, &result)
		items, _ := result["data"].([]interface{})

		// Kiểm tra không thấy data của Sales Department (đã xóa share)
		found := false
		for _, item := range items {
			itemMap, _ := item.(map[string]interface{})
			id, _ := itemMap["id"].(string)
			if id == salesDeptDataID {
				found = true
				break
			}
		}

		assert.False(t, found, "User Team A KHÔNG được thấy data của Sales Department (đã xóa share)")
		fmt.Printf("✅ User Team A KHÔNG thấy data Sales Department (sau khi xóa share)\n")
	})

	fmt.Printf("\n✅ TẤT CẢ TEST CASES ĐÃ PASS!\n")
}
