package decisionlive

import (
	"meta_commerce/internal/worker"
)

// CommandCenterWorkersSnapshot thông tin worker trên **process hiện tại** (một replica API+worker).
type CommandCenterWorkersSnapshot struct {
	// BackgroundWorkersRegistered — số worker đã đăng ký trong Registry; mỗi entry một goroutine `Start` (vòng lặp nền).
	BackgroundWorkersRegistered int `json:"backgroundWorkersRegistered"`
	// AiDecisionConsumerActive — cờ bật/tắt consumer (API override / env / mặc định true).
	AiDecisionConsumerActive bool `json:"aiDecisionConsumerActive"`
	// AiDecisionConsumerParallelSlots — pool hiệu dụng: số event tối đa xử lý song song mỗi burst (throttle có thể giảm).
	AiDecisionConsumerParallelSlots int `json:"aiDecisionConsumerParallelSlots"`
	// AiDecisionConsumerThrottled — true khi CPU/RAM đang throttle nhánh consumer (ShouldThrottle).
	AiDecisionConsumerThrottled bool `json:"aiDecisionConsumerThrottled"`
}

// BuildCommandCenterWorkersSnapshot đọc cấu hình worker runtime (không đếm goroutine thực tế từng ms).
func BuildCommandCenterWorkersSnapshot() CommandCenterWorkersSnapshot {
	p := worker.GetPriority(worker.WorkerAIDecisionConsumer, worker.PriorityCritical)
	return CommandCenterWorkersSnapshot{
		BackgroundWorkersRegistered:     worker.DefaultRegistry().Count(),
		AiDecisionConsumerActive:        worker.IsWorkerActive(worker.WorkerAIDecisionConsumer),
		AiDecisionConsumerParallelSlots: worker.EffectiveAIDecisionConsumerParallelSlots(),
		AiDecisionConsumerThrottled:     worker.ShouldThrottleWorker(worker.WorkerAIDecisionConsumer, p),
	}
}
