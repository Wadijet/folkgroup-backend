// publish_handoff — Bổ sung dòng «Bước chuyển» trong DetailBullets khi có handoff giữa AI Decision và miền nghiệp vụ.
package decisionlive

import (
	"strings"

	"meta_commerce/internal/api/aidecision/eventtypes"
)

// enrichPublishHandoffNarrative chèn (sau dòng E2E nếu có) một gạch đầu dòng mô tả bước chuyển miền — không thay thế refs/e2e.
func enrichPublishHandoffNarrative(ev *DecisionLiveEvent) {
	if ev == nil {
		return
	}
	line := handoffPublishLineVi(ev)
	if line == "" {
		return
	}
	if detailBulletsContainHandoffNarrative(ev.DetailBullets) {
		return
	}
	ev.DetailBullets = insertHandoffDetailBullet(ev.DetailBullets, line)
}

func detailBulletsContainHandoffNarrative(bullets []string) bool {
	for _, b := range bullets {
		if eventtypes.IsLiveDetailBulletHandoffNarrative(b) {
			return true
		}
	}
	return false
}

// insertHandoffDetailBullet — ưu tiên chèn ngay sau dòng «Trong quy trình:» (prependE2EPublishNarrative).
func insertHandoffDetailBullet(bullets []string, line string) []string {
	if len(bullets) == 0 {
		return []string{line}
	}
	first := strings.TrimSpace(bullets[0])
	if eventtypes.IsLiveDetailBulletE2ENarrative(first) {
		out := make([]string, 0, len(bullets)+1)
		out = append(out, bullets[0])
		out = append(out, line)
		out = append(out, bullets[1:]...)
		return out
	}
	return append([]string{line}, bullets...)
}

func handoffPublishLineVi(ev *DecisionLiveEvent) string {
	if ev.Refs != nil {
		if v := strings.TrimSpace(ev.Refs["handoffNoteVi"]); v != "" {
			return v
		}
	}
	if ev.Step != nil && ev.Step.OutputRef != nil {
		if v, ok := ev.Step.OutputRef["handoffNoteVi"].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
		if v, ok := ev.Step.OutputRef["handoffDomainVi"].(string); ok && strings.TrimSpace(v) != "" {
			return eventtypes.ResolveLiveHandoffLineFromDomainVi(v)
		}
		if jt, ok := ev.Step.OutputRef["jobType"].(string); ok {
			if line := handoffLineFromJobType(jt); line != "" {
				return line
			}
		}
	}
	return handoffLineFromAIDEventRefs(ev.Refs)
}

func handoffLineFromAIDEventRefs(refs map[string]string) string {
	if refs == nil {
		return ""
	}
	es := strings.TrimSpace(refs["eventSource"])
	et := strings.TrimSpace(refs["eventType"])
	if et == "" || es != "aidecision" {
		return ""
	}
	return eventtypes.ResolveLiveHandoffLineFromAIDEvent(es, et)
}

func handoffLineFromJobType(jt string) string {
	return eventtypes.ResolveLiveHandoffLineFromJobType(jt)
}
