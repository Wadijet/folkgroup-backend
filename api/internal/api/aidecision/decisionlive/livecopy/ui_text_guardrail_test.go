package livecopy

import (
	"os"
	"strings"
	"testing"
)

// TestUITextKhongHardcodeTrongLivecopy chặn các literal UI lõi đã chuẩn hoá ở catalog chung.
// Mục tiêu: livecopy không tái-hardcode các nhãn phase/outcome/domain/handoff thuộc decisionlive core.
func TestUITextKhongHardcodeTrongLivecopy(t *testing.T) {
	forbiddenLiterals := []string{
		"Đang chờ tới lượt xử lý",
		"Một phần không thành công",
		"Pancake / POS (pc)",
		"Đơn hàng (order)",
		"AI Decision đã tạo việc cho miền",
		"đã xếp hàng việc cho worker miền CRM (intel)",
	}
	targetFiles := []string{
		"queue.go",
		"domain_event.go",
		"ads.go",
		"queue_trace_step.go",
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
