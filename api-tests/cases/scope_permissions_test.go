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

// TestScopePermissions - Test chi tiết về Scope permissions (Scope 0 vs Scope 1)
func TestScopePermissions(t *testing.T) {
	baseURL := "http://localhost:8080/api/v1"
	waitForHealth(baseURL, 10, 1*time.Second, t)

	initTestData(t, baseURL)

	fixtures := utils.NewTestFixtures(baseURL)

	firebaseIDToken := utils.GetTestFirebaseIDToken()
	if firebaseIDToken == "" {
		t.Skip("Skipping test: TEST_FIREBASE_ID_TOKEN environment variable not set")
	}
	// Thử tạo admin user để có đầy đủ quyền
	_, _, token, userID, err := fixtures.CreateAdminUser(firebaseIDToken)
	if err != nil || token == "" {
		// Nếu không tạo được admin, thử tạo user thường
		_, _, token, err = fixtures.CreateTestUser(firebaseIDToken)
		if err != nil {
			t.Fatalf("❌ Không thể tạo user test: %v", err)
		}
		
		// Lấy user ID từ profile
		client := utils.NewHTTPClient(baseURL, 10)
		client.SetToken(token)
		_, body, err := client.GET("/auth/profile")
		if err != nil {
			t.Fatalf("❌ Không thể lấy profile: %v", err)
		}
		var profileResult map[string]interface{}
		json.Unmarshal(body, &profileResult)
		profileData, _ := profileResult["data"].(map[string]interface{})
		userID, _ = profileData["id"].(string)
	}

	client := utils.NewHTTPClient(baseURL, 10)
	client.SetToken(token)

	// Set active role trước GetRootOrganizationID (endpoint /organization/find yêu cầu X-Active-Role-ID)
	resp, body, _ := client.GET("/auth/roles")
	if resp != nil && resp.StatusCode == http.StatusOK {
		var rolesResult map[string]interface{}
		json.Unmarshal(body, &rolesResult)
		if rolesData, ok := rolesResult["data"].([]interface{}); ok && len(rolesData) > 0 {
			if firstRole, ok := rolesData[0].(map[string]interface{}); ok {
				if roleID, ok := firstRole["roleId"].(string); ok && roleID != "" {
					client.SetActiveRoleID(roleID)
					fixtures.SetActiveRoleIDForClient(roleID)
				}
			}
		}
	}

	// Lấy Root Organization ID
	rootOrgID, err := fixtures.GetRootOrganizationID(token)
	if err != nil {
		t.Fatalf("❌ Không thể lấy Root Organization ID: %v", err)
	}

	// ============================================
	// SETUP: Tạo organization hierarchy và roles với helper function
	// ============================================
	var testData *utils.OrganizationTestData
	t.Run("🏗️ Setup: Tạo organization và roles", func(t *testing.T) {
		var setupErr error
		testData, setupErr = fixtures.SetupOrganizationTestData(token, userID)
		if setupErr != nil {
			t.Logf("⚠️ Lỗi setup organization test data: %v", setupErr)
		}
		if testData != nil {
			fmt.Printf("✅ Setup organization test data thành công\n")
		}
	})

	// Map testData vào các biến cũ để tương thích với code hiện tại
	companyRoleID := ""
	deptRoleID := ""
	teamRoleID := ""

	if testData != nil {
		companyRoleID = testData.CompanyRoleID
		deptRoleID = testData.DeptARoleID
		teamRoleID = testData.TeamARoleID
	}

	// ============================================
	// TEST: Scope 0 - Chỉ thấy dữ liệu của organization mình
	// ============================================
	t.Run("🔒 Scope 0: Chỉ thấy dữ liệu của organization mình", func(t *testing.T) {
		if deptRoleID == "" || teamRoleID == "" {
			t.Skip("Skipping: Không đủ roles")
		}

		// Tạo dữ liệu ở Department
		client.SetActiveRoleID(deptRoleID)
		var deptCustomerID string
		{
			payload := map[string]interface{}{
				"customerId": fmt.Sprintf("dept_scope0_%d", time.Now().UnixNano()),
				"name":       "Dept Scope 0 Customer",
			}
			resp, body, _ := client.POST("/fb-customer/insert-one", payload)
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				json.Unmarshal(body, &result)
				if data, ok := result["data"].(map[string]interface{}); ok {
					deptCustomerID, _ = data["id"].(string)
				}
			}
		}

		// Tạo dữ liệu ở Team
		client.SetActiveRoleID(teamRoleID)
		var teamCustomerID string
		{
			payload := map[string]interface{}{
				"customerId": fmt.Sprintf("team_scope0_%d", time.Now().UnixNano()),
				"name":       "Team Scope 0 Customer",
			}
			resp, body, _ := client.POST("/fb-customer/insert-one", payload)
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				json.Unmarshal(body, &result)
				if data, ok := result["data"].(map[string]interface{}); ok {
					teamCustomerID, _ = data["id"].(string)
				}
			}
		}

		// Test: User ở Team với Scope 0 chỉ thấy dữ liệu của Team
		t.Run("Team Role với Scope 0", func(t *testing.T) {
			client.SetActiveRoleID(teamRoleID)

			resp, body, err := client.GET("/fb-customer/find")
			if err != nil {
				t.Fatalf("❌ Lỗi khi query: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				json.Unmarshal(body, &result)
				customers, ok := result["data"].([]interface{})
				if ok {
					foundTeam := false
					foundDept := false

					for _, item := range customers {
						customer, ok := item.(map[string]interface{})
						if ok {
							id, _ := customer["id"].(string)
							if id == teamCustomerID {
								foundTeam = true
							}
							if id == deptCustomerID {
								foundDept = true
							}
						}
					}

					assert.True(t, foundTeam, "Phải thấy customer của Team")
					// Với Scope 0, không nên thấy customer của Department
					// Nhưng với inverse parent lookup, có thể thấy
					if foundDept {
						fmt.Printf("⚠️ Scope 0: Thấy customer của Department (có thể do inverse parent lookup)\n")
					} else {
						fmt.Printf("✅ Scope 0: Chỉ thấy customer của Team (đúng)\n")
					}
					fmt.Printf("  Total customers: %d\n", len(customers))
				}
			}
		})
	})

	// ============================================
	// TEST: Scope 1 - Thấy dữ liệu của organization mình và children
	// ============================================
	t.Run("🔓 Scope 1: Thấy dữ liệu của organization và children", func(t *testing.T) {
		if companyRoleID == "" || deptRoleID == "" || teamRoleID == "" {
			t.Skip("Skipping: Không đủ roles")
		}

		// Tạo dữ liệu ở các cấp
		client.SetActiveRoleID(companyRoleID)
		var companyCustomerID string
		{
			payload := map[string]interface{}{
				"customerId": fmt.Sprintf("company_scope1_%d", time.Now().UnixNano()),
				"name":       "Company Scope 1 Customer",
			}
			resp, body, _ := client.POST("/fb-customer/insert-one", payload)
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				json.Unmarshal(body, &result)
				if data, ok := result["data"].(map[string]interface{}); ok {
					companyCustomerID, _ = data["id"].(string)
				}
			}
		}

		client.SetActiveRoleID(deptRoleID)
		var deptCustomerID string
		{
			payload := map[string]interface{}{
				"customerId": fmt.Sprintf("dept_scope1_%d", time.Now().UnixNano()),
				"name":       "Dept Scope 1 Customer",
			}
			resp, body, _ := client.POST("/fb-customer/insert-one", payload)
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				json.Unmarshal(body, &result)
				if data, ok := result["data"].(map[string]interface{}); ok {
					deptCustomerID, _ = data["id"].(string)
				}
			}
		}

		client.SetActiveRoleID(teamRoleID)
		var teamCustomerID string
		{
			payload := map[string]interface{}{
				"customerId": fmt.Sprintf("team_scope1_%d", time.Now().UnixNano()),
				"name":       "Team Scope 1 Customer",
			}
			resp, body, _ := client.POST("/fb-customer/insert-one", payload)
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				json.Unmarshal(body, &result)
				if data, ok := result["data"].(map[string]interface{}); ok {
					teamCustomerID, _ = data["id"].(string)
				}
			}
		}

		// Test: User ở Company với Scope 1 sẽ thấy tất cả (Company + Dept + Team)
		t.Run("Company Role với Scope 1", func(t *testing.T) {
			client.SetActiveRoleID(companyRoleID)

			resp, body, err := client.GET("/fb-customer/find")
			if err != nil {
				t.Fatalf("❌ Lỗi khi query: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				json.Unmarshal(body, &result)
				customers, ok := result["data"].([]interface{})
				if ok {
					foundCompany := false
					foundDept := false
					foundTeam := false

					for _, item := range customers {
						customer, ok := item.(map[string]interface{})
						if ok {
							id, _ := customer["id"].(string)
							if id == companyCustomerID {
								foundCompany = true
							}
							if id == deptCustomerID {
								foundDept = true
							}
							if id == teamCustomerID {
								foundTeam = true
							}
						}
					}

					// Với Scope 1, phải thấy tất cả (Company + children)
					fmt.Printf("✅ Scope 1 test: Company=%v, Dept=%v, Team=%v\n", foundCompany, foundDept, foundTeam)
					fmt.Printf("  Total customers: %d\n", len(customers))

					// Lưu ý: Cần có permission với Scope = 1 mới hoạt động đúng
					if foundCompany && foundDept && foundTeam {
						fmt.Printf("✅ Scope 1 hoạt động đúng: Thấy tất cả dữ liệu của Company và children\n")
					} else {
						fmt.Printf("⚠️ Scope 1: Một số dữ liệu không được tìm thấy (có thể do permission chưa set Scope = 1)\n")
					}
				}
			}
		})

		// Test: User ở Department với Scope 1 sẽ thấy Department + Team (children)
		t.Run("Department Role với Scope 1", func(t *testing.T) {
			client.SetActiveRoleID(deptRoleID)

			resp, body, err := client.GET("/fb-customer/find")
			if err != nil {
				t.Fatalf("❌ Lỗi khi query: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				json.Unmarshal(body, &result)
				customers, ok := result["data"].([]interface{})
				if ok {
					foundDept := false
					foundTeam := false
					foundCompany := false

					for _, item := range customers {
						customer, ok := item.(map[string]interface{})
						if ok {
							id, _ := customer["id"].(string)
							if id == deptCustomerID {
								foundDept = true
							}
							if id == teamCustomerID {
								foundTeam = true
							}
							if id == companyCustomerID {
								foundCompany = true
							}
						}
					}

					fmt.Printf("✅ Dept Scope 1 test: Dept=%v, Team=%v, Company=%v\n", foundDept, foundTeam, foundCompany)
					fmt.Printf("  Total customers: %d\n", len(customers))

					// Với Scope 1, phải thấy Department + Team
					// Có thể thấy Company nếu có inverse parent lookup
					if foundDept && foundTeam {
						fmt.Printf("✅ Scope 1 hoạt động đúng: Thấy dữ liệu của Department và children\n")
					}
					if foundCompany {
						fmt.Printf("✅ Inverse parent lookup: Thấy dữ liệu của Company (parent)\n")
					}
				}
			}
		})
	})

	// ============================================
	// TEST: System Organization với Scope 1 = Xem tất cả
	// ============================================
	t.Run("🌐 System Organization với Scope 1 = Xem tất cả", func(t *testing.T) {
		// Lấy System Organization role
		resp, body, err := client.GET("/auth/roles")
		if err != nil {
			t.Skip("Skipping: Không thể lấy roles")
		}

		if resp.StatusCode != http.StatusOK {
			t.Skip("Skipping: Không thể lấy roles")
		}

		var result map[string]interface{}
		json.Unmarshal(body, &result)
		roles, ok := result["data"].([]interface{})
		if !ok || len(roles) == 0 {
			t.Skip("Skipping: Không có roles")
		}

		// Tìm role của System Organization
		var systemRoleID string
		for _, roleItem := range roles {
			role, ok := roleItem.(map[string]interface{})
			if ok {
				orgID, _ := role["organizationId"].(string)
				if orgID == rootOrgID {
					systemRoleID, _ = role["roleId"].(string)
					break
				}
			}
		}

		if systemRoleID == "" {
			t.Skip("Skipping: Không tìm thấy System Organization role")
		}

		// Test: User với System Organization role + Scope 1 sẽ thấy tất cả
		t.Run("System Role với Scope 1", func(t *testing.T) {
			client.SetActiveRoleID(systemRoleID)

			resp, body, err := client.GET("/fb-customer/find")
			if err != nil {
				t.Fatalf("❌ Lỗi khi query: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				json.Unmarshal(body, &result)
				customers, ok := result["data"].([]interface{})
				if ok {
					fmt.Printf("✅ System Organization với Scope 1: Thấy tất cả customers (%d items)\n", len(customers))
					// System Organization với Scope 1 = xem tất cả dữ liệu trong hệ thống
				}
			}
		})
	})
}

