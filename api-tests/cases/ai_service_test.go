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

// TestAIServiceModule kiểm tra tất cả các API của Module 2 (AI Service)
func TestAIServiceModule(t *testing.T) {
	baseURL := "http://localhost:8080/api/v1"
	waitForHealth(baseURL, 10, 1*time.Second, t)

	// Sử dụng bearer token của admin
	adminToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVmN2IzOGNiZjYyZGJhMGZiMDk0Y2IiLCJ0aW1lIjoiNjk2MTk2M2MiLCJyYW5kb21OdW1iZXIiOiI0OSJ9.Y-__tpexbOJ-cg0v5PkOXUdfNLeVgvHazfjOn43bmuI"

	client := utils.NewHTTPClient(baseURL, 10)
	client.SetToken(adminToken)

	fixtures := utils.NewTestFixtures(baseURL)

	// Lấy Root Organization ID để sử dụng trong test (nếu cần)
	_, err := fixtures.GetRootOrganizationID(adminToken)
	if err != nil {
		t.Logf("⚠️ Không thể lấy Root Organization ID: %v", err)
		// Vẫn tiếp tục test, có thể sẽ fail ở phần cần organizationId
	}

	// Lấy danh sách roles của user để set active role
	resp, body, err := client.GET("/auth/roles")
	if err == nil && resp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		json.Unmarshal(body, &result)
		if data, ok := result["data"].([]interface{}); ok && len(data) > 0 {
			firstRole, ok := data[0].(map[string]interface{})
			if ok {
				roleID, ok := firstRole["roleId"].(string)
				if ok {
					client.SetActiveRoleID(roleID)
					fmt.Printf("✅ Set active role ID: %s\n", roleID)
				}
			}
		}
	}

	// ============================================
	// TEST AI WORKFLOWS
	// ============================================
	t.Run("🤖 AIWorkflows CRUD Operations", func(t *testing.T) {
		var workflowID string

		// CREATE: Tạo AI workflow
		t.Run("CREATE - Tạo AI workflow", func(t *testing.T) {
			payload := map[string]interface{}{
				"name":        fmt.Sprintf("Test Workflow %d", time.Now().UnixNano()),
				"description": "Test workflow description",
				"version":     "1.0.0",
				"rootRefType": "layer",
				"targetLevel": "L8",
				"status":      "active",
				"steps": []map[string]interface{}{
					{
						"stepId": "000000000000000000000000", // Cần stepId hợp lệ
						"order":  0,
					},
				},
			}

			resp, body, err := client.POST("/ai/workflows/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo AI workflow: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						workflowID = id
						fmt.Printf("✅ CREATE AI workflow thành công, ID: %s\n", workflowID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE AI workflow thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc AI workflow
		t.Run("READ - Đọc AI workflow", func(t *testing.T) {
			if workflowID == "" {
				t.Skip("Skipping: Chưa có workflow ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/ai/workflows/find-by-id/%s", workflowID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc AI workflow: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ READ AI workflow thành công\n")
			} else {
				t.Errorf("❌ READ AI workflow thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// UPDATE: Cập nhật AI workflow
		t.Run("UPDATE - Cập nhật AI workflow", func(t *testing.T) {
			if workflowID == "" {
				t.Skip("Skipping: Chưa có workflow ID")
			}

			payload := map[string]interface{}{
				"description": "Updated workflow description",
				"status":      "archived",
			}

			resp, body, err := client.PUT(fmt.Sprintf("/ai/workflows/update-by-id/%s", workflowID), payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi cập nhật AI workflow: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ UPDATE AI workflow thành công\n")
			} else {
				t.Errorf("❌ UPDATE AI workflow thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa AI workflow
		t.Run("DELETE - Xóa AI workflow", func(t *testing.T) {
			if workflowID == "" {
				t.Skip("Skipping: Chưa có workflow ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/ai/workflows/delete-by-id/%s", workflowID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa AI workflow: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ DELETE AI workflow thành công\n")
			} else {
				t.Errorf("❌ DELETE AI workflow thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})
	})

	// ============================================
	// TEST AI STEPS
	// ============================================
	t.Run("📝 AISteps CRUD Operations", func(t *testing.T) {
		var stepID string

		// CREATE: Tạo AI step
		t.Run("CREATE - Tạo AI step", func(t *testing.T) {
			payload := map[string]interface{}{
				"name":        fmt.Sprintf("Test Step %d", time.Now().UnixNano()),
				"description": "Test step description",
				"type":        "GENERATE",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"layer": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"outputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"candidates": map[string]interface{}{
							"type": "array",
						},
					},
				},
				"targetLevel": "L2",
				"status":      "active",
			}

			resp, body, err := client.POST("/ai/steps/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo AI step: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						stepID = id
						fmt.Printf("✅ CREATE AI step thành công, ID: %s\n", stepID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE AI step thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc AI step
		t.Run("READ - Đọc AI step", func(t *testing.T) {
			if stepID == "" {
				t.Skip("Skipping: Chưa có step ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/ai/steps/find-by-id/%s", stepID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc AI step: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ READ AI step thành công\n")
			} else {
				t.Errorf("❌ READ AI step thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// UPDATE: Cập nhật AI step
		t.Run("UPDATE - Cập nhật AI step", func(t *testing.T) {
			if stepID == "" {
				t.Skip("Skipping: Chưa có step ID")
			}

			payload := map[string]interface{}{
				"description": "Updated step description",
				"status":      "archived",
			}

			resp, body, err := client.PUT(fmt.Sprintf("/ai/steps/update-by-id/%s", stepID), payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi cập nhật AI step: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ UPDATE AI step thành công\n")
			} else {
				t.Errorf("❌ UPDATE AI step thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa AI step
		t.Run("DELETE - Xóa AI step", func(t *testing.T) {
			if stepID == "" {
				t.Skip("Skipping: Chưa có step ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/ai/steps/delete-by-id/%s", stepID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa AI step: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ DELETE AI step thành công\n")
			} else {
				t.Errorf("❌ DELETE AI step thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})
	})

	// ============================================
	// TEST AI PROMPT TEMPLATES
	// ============================================
	t.Run("📋 AIPromptTemplates CRUD Operations", func(t *testing.T) {
		var promptTemplateID string

		// CREATE: Tạo AI prompt template
		t.Run("CREATE - Tạo AI prompt template", func(t *testing.T) {
			payload := map[string]interface{}{
				"name":        fmt.Sprintf("Test Prompt Template %d", time.Now().UnixNano()),
				"description": "Test prompt template description",
				"type":        "generate",
				"version":     "1.0.0",
				"prompt":      "Generate content for {{layer}}",
				"provider":    "openai",
				"model":       "gpt-4",
				"status":      "active",
			}

			resp, body, err := client.POST("/ai/prompt-templates/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo AI prompt template: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						promptTemplateID = id
						fmt.Printf("✅ CREATE AI prompt template thành công, ID: %s\n", promptTemplateID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE AI prompt template thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc AI prompt template
		t.Run("READ - Đọc AI prompt template", func(t *testing.T) {
			if promptTemplateID == "" {
				t.Skip("Skipping: Chưa có prompt template ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/ai/prompt-templates/find-by-id/%s", promptTemplateID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc AI prompt template: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ READ AI prompt template thành công\n")
			} else {
				t.Errorf("❌ READ AI prompt template thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// UPDATE: Cập nhật AI prompt template
		t.Run("UPDATE - Cập nhật AI prompt template", func(t *testing.T) {
			if promptTemplateID == "" {
				t.Skip("Skipping: Chưa có prompt template ID")
			}

			payload := map[string]interface{}{
				"description": "Updated prompt template description",
				"status":      "archived",
			}

			resp, body, err := client.PUT(fmt.Sprintf("/ai/prompt-templates/update-by-id/%s", promptTemplateID), payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi cập nhật AI prompt template: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ UPDATE AI prompt template thành công\n")
			} else {
				t.Errorf("❌ UPDATE AI prompt template thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa AI prompt template
		t.Run("DELETE - Xóa AI prompt template", func(t *testing.T) {
			if promptTemplateID == "" {
				t.Skip("Skipping: Chưa có prompt template ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/ai/prompt-templates/delete-by-id/%s", promptTemplateID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa AI prompt template: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ DELETE AI prompt template thành công\n")
			} else {
				t.Errorf("❌ DELETE AI prompt template thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})
	})

	// ============================================
	// TEST AI WORKFLOW RUNS
	// ============================================
	t.Run("🔄 AIWorkflowRuns CRUD Operations", func(t *testing.T) {
		var workflowRunID string

		// CREATE: Tạo AI workflow run
		t.Run("CREATE - Tạo AI workflow run", func(t *testing.T) {
			payload := map[string]interface{}{
				"workflowId":  "000000000000000000000000", // Cần workflowId hợp lệ
				"rootRefId":   "000000000000000000000000",
				"rootRefType": "layer",
				"status":      "pending",
			}

			resp, body, err := client.POST("/ai/workflow-runs/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo AI workflow run: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						workflowRunID = id
						fmt.Printf("✅ CREATE AI workflow run thành công, ID: %s\n", workflowRunID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE AI workflow run thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc AI workflow run
		t.Run("READ - Đọc AI workflow run", func(t *testing.T) {
			if workflowRunID == "" {
				t.Skip("Skipping: Chưa có workflow run ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/ai/workflow-runs/find-by-id/%s", workflowRunID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc AI workflow run: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ READ AI workflow run thành công\n")
			} else {
				t.Errorf("❌ READ AI workflow run thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// UPDATE: Cập nhật AI workflow run
		t.Run("UPDATE - Cập nhật AI workflow run", func(t *testing.T) {
			if workflowRunID == "" {
				t.Skip("Skipping: Chưa có workflow run ID")
			}

			payload := map[string]interface{}{
				"status": "running",
			}

			resp, body, err := client.PUT(fmt.Sprintf("/ai/workflow-runs/update-by-id/%s", workflowRunID), payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi cập nhật AI workflow run: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ UPDATE AI workflow run thành công\n")
			} else {
				t.Errorf("❌ UPDATE AI workflow run thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa AI workflow run
		t.Run("DELETE - Xóa AI workflow run", func(t *testing.T) {
			if workflowRunID == "" {
				t.Skip("Skipping: Chưa có workflow run ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/ai/workflow-runs/delete-by-id/%s", workflowRunID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa AI workflow run: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ DELETE AI workflow run thành công\n")
			} else {
				t.Errorf("❌ DELETE AI workflow run thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})
	})

	// ============================================
	// TEST AI STEP RUNS
	// ============================================
	t.Run("⚙️ AIStepRuns CRUD Operations", func(t *testing.T) {
		var stepRunID string

		// CREATE: Tạo AI step run
		t.Run("CREATE - Tạo AI step run", func(t *testing.T) {
			payload := map[string]interface{}{
				"workflowRunId": "000000000000000000000000", // Cần workflowRunId hợp lệ
				"stepId":        "000000000000000000000000", // Cần stepId hợp lệ
				"order":         0,
				"status":        "pending",
			}

			resp, body, err := client.POST("/ai/step-runs/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo AI step run: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						stepRunID = id
						fmt.Printf("✅ CREATE AI step run thành công, ID: %s\n", stepRunID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE AI step run thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc AI step run
		t.Run("READ - Đọc AI step run", func(t *testing.T) {
			if stepRunID == "" {
				t.Skip("Skipping: Chưa có step run ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/ai/step-runs/find-by-id/%s", stepRunID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc AI step run: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ READ AI step run thành công\n")
			} else {
				t.Errorf("❌ READ AI step run thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// UPDATE: Cập nhật AI step run
		t.Run("UPDATE - Cập nhật AI step run", func(t *testing.T) {
			if stepRunID == "" {
				t.Skip("Skipping: Chưa có step run ID")
			}

			payload := map[string]interface{}{
				"status": "running",
			}

			resp, body, err := client.PUT(fmt.Sprintf("/ai/step-runs/update-by-id/%s", stepRunID), payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi cập nhật AI step run: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ UPDATE AI step run thành công\n")
			} else {
				t.Errorf("❌ UPDATE AI step run thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa AI step run
		t.Run("DELETE - Xóa AI step run", func(t *testing.T) {
			if stepRunID == "" {
				t.Skip("Skipping: Chưa có step run ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/ai/step-runs/delete-by-id/%s", stepRunID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa AI step run: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ DELETE AI step run thành công\n")
			} else {
				t.Errorf("❌ DELETE AI step run thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})
	})

	// ============================================
	// TEST AI GENERATION BATCHES
	// ============================================
	t.Run("📦 AIGenerationBatches CRUD Operations", func(t *testing.T) {
		var batchID string

		// CREATE: Tạo AI generation batch
		t.Run("CREATE - Tạo AI generation batch", func(t *testing.T) {
			payload := map[string]interface{}{
				"stepRunId":   "000000000000000000000000", // Cần stepRunId hợp lệ
				"targetCount": 5,
				"status":      "pending",
			}

			resp, body, err := client.POST("/ai/generation-batches/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo AI generation batch: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						batchID = id
						fmt.Printf("✅ CREATE AI generation batch thành công, ID: %s\n", batchID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE AI generation batch thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc AI generation batch
		t.Run("READ - Đọc AI generation batch", func(t *testing.T) {
			if batchID == "" {
				t.Skip("Skipping: Chưa có batch ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/ai/generation-batches/find-by-id/%s", batchID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc AI generation batch: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ READ AI generation batch thành công\n")
			} else {
				t.Errorf("❌ READ AI generation batch thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// UPDATE: Cập nhật AI generation batch
		t.Run("UPDATE - Cập nhật AI generation batch", func(t *testing.T) {
			if batchID == "" {
				t.Skip("Skipping: Chưa có batch ID")
			}

			payload := map[string]interface{}{
				"status":     "generating",
				"actualCount": 3,
			}

			resp, body, err := client.PUT(fmt.Sprintf("/ai/generation-batches/update-by-id/%s", batchID), payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi cập nhật AI generation batch: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ UPDATE AI generation batch thành công\n")
			} else {
				t.Errorf("❌ UPDATE AI generation batch thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa AI generation batch
		t.Run("DELETE - Xóa AI generation batch", func(t *testing.T) {
			if batchID == "" {
				t.Skip("Skipping: Chưa có batch ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/ai/generation-batches/delete-by-id/%s", batchID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa AI generation batch: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ DELETE AI generation batch thành công\n")
			} else {
				t.Errorf("❌ DELETE AI generation batch thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})
	})

	// ============================================
	// TEST AI CANDIDATES
	// ============================================
	t.Run("🎯 AICandidates CRUD Operations", func(t *testing.T) {
		var candidateID string

		// CREATE: Tạo AI candidate
		t.Run("CREATE - Tạo AI candidate", func(t *testing.T) {
			payload := map[string]interface{}{
				"generationBatchId": "000000000000000000000000", // Cần generationBatchId hợp lệ
				"stepRunId":         "000000000000000000000000", // Cần stepRunId hợp lệ
				"text":              fmt.Sprintf("Test Candidate Text %d", time.Now().UnixNano()),
				"createdByAIRunId":  "000000000000000000000000", // Cần createdByAIRunId hợp lệ
			}

			resp, body, err := client.POST("/ai/candidates/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo AI candidate: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						candidateID = id
						fmt.Printf("✅ CREATE AI candidate thành công, ID: %s\n", candidateID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE AI candidate thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc AI candidate
		t.Run("READ - Đọc AI candidate", func(t *testing.T) {
			if candidateID == "" {
				t.Skip("Skipping: Chưa có candidate ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/ai/candidates/find-by-id/%s", candidateID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc AI candidate: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ READ AI candidate thành công\n")
			} else {
				t.Errorf("❌ READ AI candidate thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// UPDATE: Cập nhật AI candidate
		t.Run("UPDATE - Cập nhật AI candidate", func(t *testing.T) {
			if candidateID == "" {
				t.Skip("Skipping: Chưa có candidate ID")
			}

			judgeScore := 0.85
			selected := true
			payload := map[string]interface{}{
				"judgeScore": &judgeScore,
				"selected":   &selected,
			}

			resp, body, err := client.PUT(fmt.Sprintf("/ai/candidates/update-by-id/%s", candidateID), payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi cập nhật AI candidate: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ UPDATE AI candidate thành công\n")
			} else {
				t.Errorf("❌ UPDATE AI candidate thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa AI candidate
		t.Run("DELETE - Xóa AI candidate", func(t *testing.T) {
			if candidateID == "" {
				t.Skip("Skipping: Chưa có candidate ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/ai/candidates/delete-by-id/%s", candidateID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa AI candidate: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ DELETE AI candidate thành công\n")
			} else {
				t.Errorf("❌ DELETE AI candidate thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})
	})

	// ============================================
	// TEST AI RUNS
	// ============================================
	t.Run("🚀 AIRuns CRUD Operations", func(t *testing.T) {
		var aiRunID string

		// CREATE: Tạo AI run
		t.Run("CREATE - Tạo AI run", func(t *testing.T) {
			payload := map[string]interface{}{
				"type":     "GENERATE",
				"provider": "openai",
				"model":    "gpt-4",
				"prompt":   "Generate content for layer",
				"status":   "pending",
			}

			resp, body, err := client.POST("/ai/ai-runs/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo AI run: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						aiRunID = id
						fmt.Printf("✅ CREATE AI run thành công, ID: %s\n", aiRunID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE AI run thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc AI run
		t.Run("READ - Đọc AI run", func(t *testing.T) {
			if aiRunID == "" {
				t.Skip("Skipping: Chưa có AI run ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/ai/ai-runs/find-by-id/%s", aiRunID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc AI run: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ READ AI run thành công\n")
			} else {
				t.Errorf("❌ READ AI run thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// UPDATE: Cập nhật AI run
		t.Run("UPDATE - Cập nhật AI run", func(t *testing.T) {
			if aiRunID == "" {
				t.Skip("Skipping: Chưa có AI run ID")
			}

			cost := 0.01
			latency := int64(1500)
			payload := map[string]interface{}{
				"status":  "completed",
				"cost":    &cost,
				"latency": &latency,
			}

			resp, body, err := client.PUT(fmt.Sprintf("/ai/ai-runs/update-by-id/%s", aiRunID), payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi cập nhật AI run: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ UPDATE AI run thành công\n")
			} else {
				t.Errorf("❌ UPDATE AI run thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa AI run
		t.Run("DELETE - Xóa AI run", func(t *testing.T) {
			if aiRunID == "" {
				t.Skip("Skipping: Chưa có AI run ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/ai/ai-runs/delete-by-id/%s", aiRunID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa AI run: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ DELETE AI run thành công\n")
			} else {
				t.Errorf("❌ DELETE AI run thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})
	})

	// ============================================
	// TEST AI WORKFLOW COMMANDS
	// ============================================
	t.Run("📨 AIWorkflowCommands CRUD Operations", func(t *testing.T) {
		var commandID string

		// CREATE: Tạo AI workflow command
		t.Run("CREATE - Tạo AI workflow command", func(t *testing.T) {
			payload := map[string]interface{}{
				"commandType": "START_WORKFLOW",
				"workflowId":  "000000000000000000000000", // Cần workflowId hợp lệ
				"rootRefId":   "000000000000000000000000",
				"rootRefType": "layer",
				"status":      "pending",
			}

			resp, body, err := client.POST("/ai/workflow-commands/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo AI workflow command: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						commandID = id
						fmt.Printf("✅ CREATE AI workflow command thành công, ID: %s\n", commandID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE AI workflow command thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc AI workflow command
		t.Run("READ - Đọc AI workflow command", func(t *testing.T) {
			if commandID == "" {
				t.Skip("Skipping: Chưa có command ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/ai/workflow-commands/find-by-id/%s", commandID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc AI workflow command: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ READ AI workflow command thành công\n")
			} else {
				t.Errorf("❌ READ AI workflow command thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// UPDATE: Cập nhật AI workflow command
		t.Run("UPDATE - Cập nhật AI workflow command", func(t *testing.T) {
			if commandID == "" {
				t.Skip("Skipping: Chưa có command ID")
			}

			payload := map[string]interface{}{
				"status": "executing",
			}

			resp, body, err := client.PUT(fmt.Sprintf("/ai/workflow-commands/update-by-id/%s", commandID), payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi cập nhật AI workflow command: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ UPDATE AI workflow command thành công\n")
			} else {
				t.Errorf("❌ UPDATE AI workflow command thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa AI workflow command
		t.Run("DELETE - Xóa AI workflow command", func(t *testing.T) {
			if commandID == "" {
				t.Skip("Skipping: Chưa có command ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/ai/workflow-commands/delete-by-id/%s", commandID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa AI workflow command: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				assert.Equal(t, "success", result["status"], "Status phải là success")
				fmt.Printf("✅ DELETE AI workflow command thành công\n")
			} else {
				t.Errorf("❌ DELETE AI workflow command thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})
	})
}
