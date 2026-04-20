package decisionlive

import (
	"os"
	"strings"
	"testing"
)

// TestUITextKhongHardcodeTrongDecisionLive đảm bảo text UI đã chuyển về catalog eventtypes,
// tránh quay lại hardcode tại các file enrich/publish trọng yếu.
func TestUITextKhongHardcodeTrongDecisionLive(t *testing.T) {
	forbiddenLiterals := []string{
		"Đang chờ tới lượt xử lý",
		"Một phần không thành công",
		"Pancake / POS (pc)",
		"Đơn hàng (order)",
		"AI Decision đã tạo việc cho miền",
		"đã xếp hàng việc cho worker miền CRM (intel)",
	}
	targetFiles := []string{
		"outcome.go",
		"feed_source_enrich.go",
		"business_domain_enrich.go",
		"persist_org_audit.go",
		"publish.go",
		"publish_handoff.go",
	}

	for _, file := range targetFiles {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("không đọc được file %s: %v", file, err)
		}
		content := string(raw)
		for _, literal := range forbiddenLiterals {
			if strings.Contains(content, literal) {
				t.Fatalf("phát hiện hardcode UI trong %s: %q", file, literal)
			}
		}
	}
}
