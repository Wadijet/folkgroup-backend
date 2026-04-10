package decisionlive

// Giới hạn kích thước processTrace khi Publish (tránh payload WS/DB quá lớn).
const (
	maxProcessTraceNodes = 64
	maxProcessTraceDepth = 12
)

func countProcessTraceNodesAll(nodes []DecisionLiveProcessNode) int {
	n := 0
	for _, node := range nodes {
		n++
		n += countProcessTraceNodesAll(node.Children)
	}
	return n
}

func capProcessTraceNodes(nodes []DecisionLiveProcessNode, depth, budget int) ([]DecisionLiveProcessNode, int) {
	if budget <= 0 || depth > maxProcessTraceDepth {
		return nil, budget
	}
	out := make([]DecisionLiveProcessNode, 0, len(nodes))
	for _, node := range nodes {
		if budget <= 0 {
			break
		}
		capped := node
		budget--
		var childBudget int
		capped.Children, childBudget = capProcessTraceNodes(node.Children, depth+1, budget)
		budget = childBudget
		out = append(out, capped)
	}
	return out, budget
}

// CapDecisionLiveProcessTrace — Cắt bớt cây processTrace theo ngân sách nút; gọi từ Publish.
func CapDecisionLiveProcessTrace(ev *DecisionLiveEvent) {
	if ev == nil || len(ev.ProcessTrace) == 0 {
		return
	}
	if countProcessTraceNodesAll(ev.ProcessTrace) <= maxProcessTraceNodes {
		return
	}
	capped, _ := capProcessTraceNodes(ev.ProcessTrace, 0, maxProcessTraceNodes)
	ev.ProcessTrace = capped
}
