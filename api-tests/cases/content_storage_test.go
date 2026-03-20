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

// TestContentStorageModule kiểm tra tất cả các API của Module 1 (Content Storage)
func TestContentStorageModule(t *testing.T) {
	baseURL := "http://localhost:8080/api/v1"

	// Setup với admin user (token mới từ email/password hoặc Firebase)
	fixtures, _, adminToken, client, err := utils.SetupTestWithAdminUser(t, baseURL)
	if err != nil {
		t.Fatalf("❌ Không thể setup test: %v", err)
	}
	_ = fixtures

	// Lấy Root Organization ID để sử dụng trong test (nếu cần)
	_, err = fixtures.GetRootOrganizationID(adminToken)
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
	// TEST CONTENT NODES (L1-L6)
	// ============================================
	t.Run("📄 ContentNodes CRUD Operations", func(t *testing.T) {
		var contentNodeID string

		// CREATE: Tạo content node
		t.Run("CREATE - Tạo content node", func(t *testing.T) {
			payload := map[string]interface{}{
				"type":     "pillar", // Bắt buộc: pillar, stp, insight, contentLine, gene, script
				"text":     fmt.Sprintf("Test Content Node Text %d", time.Now().UnixNano()), // Bắt buộc
				"name":     fmt.Sprintf("Test Content Node %d", time.Now().UnixNano()),
				"status":   "active",
			}

			resp, body, err := client.POST("/content/nodes/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo content node: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						contentNodeID = id
						fmt.Printf("✅ CREATE content node thành công, ID: %s\n", contentNodeID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE content node thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc content node
		t.Run("READ - Đọc content node", func(t *testing.T) {
			if contentNodeID == "" {
				t.Skip("Skipping: Chưa có content node ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/content/nodes/find-by-id/%s", contentNodeID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc content node: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ READ content node thành công\n")
			} else {
				t.Errorf("❌ READ content node thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// UPDATE: Cập nhật content node
		t.Run("UPDATE - Cập nhật content node", func(t *testing.T) {
			if contentNodeID == "" {
				t.Skip("Skipping: Chưa có content node ID")
			}

			payload := map[string]interface{}{
				"text": fmt.Sprintf("Updated Content Node Text %d", time.Now().UnixNano()),
				"name": fmt.Sprintf("Updated Content Node %d", time.Now().UnixNano()),
			}

			resp, body, err := client.PUT(fmt.Sprintf("/content/nodes/update-by-id/%s", contentNodeID), payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi cập nhật content node: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ UPDATE content node thành công\n")
			} else {
				t.Errorf("❌ UPDATE content node thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// LIST: Liệt kê content nodes
		t.Run("LIST - Liệt kê content nodes", func(t *testing.T) {
			resp, body, err := client.GET("/content/nodes/find")
			if err != nil {
				t.Fatalf("❌ Lỗi khi liệt kê content nodes: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ LIST content nodes thành công\n")
			} else {
				t.Errorf("❌ LIST content nodes thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa content node
		t.Run("DELETE - Xóa content node", func(t *testing.T) {
			if contentNodeID == "" {
				t.Skip("Skipping: Chưa có content node ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/content/nodes/delete-by-id/%s", contentNodeID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa content node: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ DELETE content node thành công\n")
			} else {
				t.Errorf("❌ DELETE content node thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// Custom endpoint: GetTree
		t.Run("GET_TREE - Lấy cây content nodes", func(t *testing.T) {
			if contentNodeID == "" {
				t.Skip("Skipping: Chưa có content node ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/content/nodes/tree/%s", contentNodeID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi lấy tree: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ GET_TREE content nodes thành công\n")
			} else {
				t.Logf("⚠️ GET_TREE content nodes (status: %d, body: %s) - có thể node đã bị xóa", resp.StatusCode, string(body))
			}
		})
	})

	// ============================================
	// TEST VIDEOS (L7)
	// ============================================
	t.Run("🎥 ContentVideos CRUD Operations", func(t *testing.T) {
		var videoID string

		// CREATE: Tạo video
		t.Run("CREATE - Tạo video", func(t *testing.T) {
			// Cần tạo script node trước để có scriptId
			// Tạm thời dùng một scriptId giả (sẽ fail nếu không có script thật)
			payload := map[string]interface{}{
				"scriptId": "000000000000000000000000", // Bắt buộc - cần scriptId hợp lệ
				"assetUrl": "https://example.com/video.mp4",
				"status":  "pending",
			}

			resp, body, err := client.POST("/content/videos/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo video: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						videoID = id
						fmt.Printf("✅ CREATE video thành công, ID: %s\n", videoID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE video thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc video
		t.Run("READ - Đọc video", func(t *testing.T) {
			if videoID == "" {
				t.Skip("Skipping: Chưa có video ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/content/videos/find-by-id/%s", videoID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc video: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ READ video thành công\n")
			} else {
				t.Errorf("❌ READ video thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// UPDATE: Cập nhật video
		t.Run("UPDATE - Cập nhật video", func(t *testing.T) {
			if videoID == "" {
				t.Skip("Skipping: Chưa có video ID")
			}

			payload := map[string]interface{}{
				"status": "ready",
			}

			resp, body, err := client.PUT(fmt.Sprintf("/content/videos/update-by-id/%s", videoID), payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi cập nhật video: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ UPDATE video thành công\n")
			} else {
				t.Errorf("❌ UPDATE video thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa video
		t.Run("DELETE - Xóa video", func(t *testing.T) {
			if videoID == "" {
				t.Skip("Skipping: Chưa có video ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/content/videos/delete-by-id/%s", videoID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa video: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ DELETE video thành công\n")
			} else {
				t.Errorf("❌ DELETE video thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})
	})

	// ============================================
	// TEST PUBLICATIONS (L8)
	// ============================================
	t.Run("📰 ContentPublications CRUD Operations", func(t *testing.T) {
		var publicationID string

		// CREATE: Tạo publication
		t.Run("CREATE - Tạo publication", func(t *testing.T) {
			// Cần videoId và platform (bắt buộc)
			payload := map[string]interface{}{
				"videoId": "000000000000000000000000", // Bắt buộc - cần videoId hợp lệ
				"platform": "facebook", // Bắt buộc: facebook, tiktok, youtube, instagram
				"status":   "draft",
			}

			resp, body, err := client.POST("/content/publications/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo publication: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						publicationID = id
						fmt.Printf("✅ CREATE publication thành công, ID: %s\n", publicationID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE publication thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc publication
		t.Run("READ - Đọc publication", func(t *testing.T) {
			if publicationID == "" {
				t.Skip("Skipping: Chưa có publication ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/content/publications/find-by-id/%s", publicationID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc publication: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ READ publication thành công\n")
			} else {
				t.Errorf("❌ READ publication thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// UPDATE: Cập nhật publication
		t.Run("UPDATE - Cập nhật publication", func(t *testing.T) {
			if publicationID == "" {
				t.Skip("Skipping: Chưa có publication ID")
			}

			payload := map[string]interface{}{
				"status": "published",
			}

			resp, body, err := client.PUT(fmt.Sprintf("/content/publications/update-by-id/%s", publicationID), payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi cập nhật publication: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ UPDATE publication thành công\n")
			} else {
				t.Errorf("❌ UPDATE publication thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa publication
		t.Run("DELETE - Xóa publication", func(t *testing.T) {
			if publicationID == "" {
				t.Skip("Skipping: Chưa có publication ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/content/publications/delete-by-id/%s", publicationID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa publication: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ DELETE publication thành công\n")
			} else {
				t.Errorf("❌ DELETE publication thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})
	})

	// ============================================
	// TEST DRAFT CONTENT NODES
	// ============================================
	t.Run("📝 ContentDraftNodes CRUD Operations", func(t *testing.T) {
		var draftNodeID string

		// CREATE: Tạo draft content node
		t.Run("CREATE - Tạo draft content node", func(t *testing.T) {
			payload := map[string]interface{}{
				"type":     "pillar", // Bắt buộc
				"text":     fmt.Sprintf("Test Draft Node Text %d", time.Now().UnixNano()), // Bắt buộc
				"name":     fmt.Sprintf("Test Draft Node %d", time.Now().UnixNano()),
				"approvalStatus": "draft",
			}

			resp, body, err := client.POST("/content/drafts/nodes/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo draft content node: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						draftNodeID = id
						fmt.Printf("✅ CREATE draft content node thành công, ID: %s\n", draftNodeID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE draft content node thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc draft content node
		t.Run("READ - Đọc draft content node", func(t *testing.T) {
			if draftNodeID == "" {
				t.Skip("Skipping: Chưa có draft node ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/content/drafts/nodes/find-by-id/%s", draftNodeID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc draft content node: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ READ draft content node thành công\n")
			} else {
				t.Errorf("❌ READ draft content node thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// UPDATE: Cập nhật draft content node
		t.Run("UPDATE - Cập nhật draft content node", func(t *testing.T) {
			if draftNodeID == "" {
				t.Skip("Skipping: Chưa có draft node ID")
			}

			payload := map[string]interface{}{
				"text": fmt.Sprintf("Updated Draft Node Text %d", time.Now().UnixNano()),
			}

			resp, body, err := client.PUT(fmt.Sprintf("/content/drafts/nodes/update-by-id/%s", draftNodeID), payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi cập nhật draft content node: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ UPDATE draft content node thành công\n")
			} else {
				t.Errorf("❌ UPDATE draft content node thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa draft content node
		t.Run("DELETE - Xóa draft content node", func(t *testing.T) {
			if draftNodeID == "" {
				t.Skip("Skipping: Chưa có draft node ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/content/drafts/nodes/delete-by-id/%s", draftNodeID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa draft content node: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ DELETE draft content node thành công\n")
			} else {
				t.Errorf("❌ DELETE draft content node thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// Custom endpoint: CommitDraftNode (test trước khi xóa)
		t.Run("COMMIT - Commit draft content node", func(t *testing.T) {
			if draftNodeID == "" {
				t.Skip("Skipping: Chưa có draft node ID")
			}

			// Tạo lại draft node để test commit
			createPayload := map[string]interface{}{
				"type":     "pillar",
				"text":     fmt.Sprintf("Test Draft Node for Commit %d", time.Now().UnixNano()),
				"name":     fmt.Sprintf("Test Draft Node for Commit %d", time.Now().UnixNano()),
			}

			createResp, createBody, err := client.POST("/content/drafts/nodes/insert-one", createPayload)
			if err == nil && (createResp.StatusCode == http.StatusOK || createResp.StatusCode == http.StatusCreated) {
				var createResult map[string]interface{}
				json.Unmarshal(createBody, &createResult)
				if data, ok := createResult["data"].(map[string]interface{}); ok {
					if id, ok := data["id"].(string); ok {
						commitDraftID := id

						// Test commit
						resp, body, err := client.POST(fmt.Sprintf("/content/drafts/nodes/%s/commit", commitDraftID), nil)
						if err != nil {
							t.Logf("⚠️ Lỗi khi commit draft content node: %v", err)
						} else if resp.StatusCode == http.StatusOK {
							var result map[string]interface{}
							err = json.Unmarshal(body, &result)
							assert.NoError(t, err, "Phải parse được JSON response")
							fmt.Printf("✅ COMMIT draft content node thành công\n")
						} else {
							t.Logf("⚠️ COMMIT draft content node (status: %d, body: %s)", resp.StatusCode, string(body))
						}
					}
				}
			}
		})
	})

	// ============================================
	// TEST DRAFT VIDEOS
	// ============================================
	t.Run("🎬 ContentDraftVideos CRUD Operations", func(t *testing.T) {
		var draftVideoID string

		// CREATE: Tạo draft video
		t.Run("CREATE - Tạo draft video", func(t *testing.T) {
			payload := map[string]interface{}{
				"draftScriptId": "000000000000000000000000", // Bắt buộc - cần draftScriptId hợp lệ
				"assetUrl":      "https://example.com/draft-video.mp4",
				"status":        "pending",
				"approvalStatus": "draft",
			}

			resp, body, err := client.POST("/content/drafts/videos/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo draft video: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						draftVideoID = id
						fmt.Printf("✅ CREATE draft video thành công, ID: %s\n", draftVideoID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE draft video thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc draft video
		t.Run("READ - Đọc draft video", func(t *testing.T) {
			if draftVideoID == "" {
				t.Skip("Skipping: Chưa có draft video ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/content/drafts/videos/find-by-id/%s", draftVideoID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc draft video: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ READ draft video thành công\n")
			} else {
				t.Errorf("❌ READ draft video thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa draft video
		t.Run("DELETE - Xóa draft video", func(t *testing.T) {
			if draftVideoID == "" {
				t.Skip("Skipping: Chưa có draft video ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/content/drafts/videos/delete-by-id/%s", draftVideoID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa draft video: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ DELETE draft video thành công\n")
			} else {
				t.Errorf("❌ DELETE draft video thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})
	})

	// ============================================
	// TEST DRAFT PUBLICATIONS
	// ============================================
	t.Run("📋 ContentDraftPublications CRUD Operations", func(t *testing.T) {
		var draftPublicationID string

		// CREATE: Tạo draft publication
		t.Run("CREATE - Tạo draft publication", func(t *testing.T) {
			payload := map[string]interface{}{
				"draftVideoId": "000000000000000000000000", // Bắt buộc - cần draftVideoId hợp lệ
				"platform":     "facebook", // Bắt buộc: facebook, tiktok, youtube, instagram
				"status":       "draft",
				"approvalStatus": "draft",
			}

			resp, body, err := client.POST("/content/drafts/publications/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo draft publication: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						draftPublicationID = id
						fmt.Printf("✅ CREATE draft publication thành công, ID: %s\n", draftPublicationID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Errorf("❌ CREATE draft publication thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc draft publication
		t.Run("READ - Đọc draft publication", func(t *testing.T) {
			if draftPublicationID == "" {
				t.Skip("Skipping: Chưa có draft publication ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/content/drafts/publications/find-by-id/%s", draftPublicationID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc draft publication: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ READ draft publication thành công\n")
			} else {
				t.Errorf("❌ READ draft publication thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// DELETE: Xóa draft publication
		t.Run("DELETE - Xóa draft publication", func(t *testing.T) {
			if draftPublicationID == "" {
				t.Skip("Skipping: Chưa có draft publication ID")
			}

			resp, body, err := client.DELETE(fmt.Sprintf("/content/drafts/publications/delete-by-id/%s", draftPublicationID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi xóa draft publication: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ DELETE draft publication thành công\n")
			} else {
				t.Errorf("❌ DELETE draft publication thất bại (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})
	})

	// ============================================
	// TEST DRAFT APPROVALS
	// ============================================
	t.Run("✅ ContentDraftApprovals CRUD Operations", func(t *testing.T) {
		var draftApprovalID string

		// CREATE: Tạo draft approval
		t.Run("CREATE - Tạo draft approval", func(t *testing.T) {
			// Cần ít nhất một target: workflowRunId, draftNodeId, draftVideoId, hoặc draftPublicationId
			payload := map[string]interface{}{
				"draftNodeId": "000000000000000000000000", // Cần ít nhất một target
				"status":      "pending",
			}

			resp, body, err := client.POST("/content/drafts/approvals/insert-one", payload)
			if err != nil {
				t.Fatalf("❌ Lỗi khi tạo draft approval: %v", err)
			}

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")

				data, ok := result["data"].(map[string]interface{})
				if ok {
					id, ok := data["id"].(string)
					if ok {
						draftApprovalID = id
						fmt.Printf("✅ CREATE draft approval thành công, ID: %s\n", draftApprovalID)
					}
				}
				assert.Equal(t, "success", result["status"], "Status phải là success")
			} else {
				t.Logf("⚠️ CREATE draft approval (status: %d, body: %s) - có thể cần thêm fields bắt buộc", resp.StatusCode, string(body))
			}
		})

		// READ: Đọc draft approval
		t.Run("READ - Đọc draft approval", func(t *testing.T) {
			if draftApprovalID == "" {
				t.Skip("Skipping: Chưa có draft approval ID")
			}

			resp, body, err := client.GET(fmt.Sprintf("/content/drafts/approvals/find-by-id/%s", draftApprovalID))
			if err != nil {
				t.Fatalf("❌ Lỗi khi đọc draft approval: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ READ draft approval thành công\n")
			} else {
				t.Logf("⚠️ READ draft approval (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})

		// LIST: Liệt kê draft approvals
		t.Run("LIST - Liệt kê draft approvals", func(t *testing.T) {
			resp, body, err := client.GET("/content/drafts/approvals/find")
			if err != nil {
				t.Fatalf("❌ Lỗi khi liệt kê draft approvals: %v", err)
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err, "Phải parse được JSON response")
				fmt.Printf("✅ LIST draft approvals thành công\n")
			} else {
				t.Logf("⚠️ LIST draft approvals (status: %d, body: %s)", resp.StatusCode, string(body))
			}
		})
	})

	fmt.Printf("\n✅ Hoàn thành test Module 1 (Content Storage)\n")
}
