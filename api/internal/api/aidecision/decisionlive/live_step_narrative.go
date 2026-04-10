package decisionlive

import "strings"

// Tiền tố cố định — khớp docs/flows/bang-pha-buoc-event-e2e.md (khung sáu trường; label = LabelVi / Title riêng).
const (
	LiveStepPrefixPurpose = "Mục đích: "
	LiveStepPrefixInput   = "Đầu vào: "
	LiveStepPrefixLogic   = "Đã xét: "
	LiveStepPrefixResult  = "Kết quả: "
	LiveStepPrefixNext    = "Tiếp theo: "
)

// FormatLiveStepNarrativeVi ghép nội dung một bước timeline/processTrace theo khung: purpose, inputSummary, logicSummary, resultSummary, nextStepHint.
// Phần rỗng (sau trim) bị bỏ; các dòng cách nhau bằng xuống dòng để UI/log dễ đọc.
func FormatLiveStepNarrativeVi(purpose, inputSummary, logicSummary, resultSummary, nextStepHint string) string {
	var b strings.Builder
	add := func(prefix, body string) {
		body = strings.TrimSpace(body)
		if body == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(prefix)
		b.WriteString(body)
	}
	add(LiveStepPrefixPurpose, purpose)
	add(LiveStepPrefixInput, inputSummary)
	add(LiveStepPrefixLogic, logicSummary)
	add(LiveStepPrefixResult, resultSummary)
	add(LiveStepPrefixNext, nextStepHint)
	return b.String()
}
