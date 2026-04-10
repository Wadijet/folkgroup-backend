// Package eventtypes — Catalog pha/bước E2E (G1–G6) cho frontend tra cứu; khớp docs/flows/bang-pha-buoc-event-e2e.md §5.2–5.3.
// G1 = pha ghi thô (CIO ingress → L1 → enqueue). G2 = consumer queue + merge L2. G3…G6 = intel, ra quyết định, thực thi, học.
package eventtypes

// E2ECatalogSchemaVersion — tăng khi đổi hình dạng JSON hoặc cắt bớt cột (client cache).
const E2ECatalogSchemaVersion = 10 // v10: steps — descriptionTechnicalVi + descriptionUserVi; stages — userSummaryVi (bỏ shortVi).

// E2EStageCatalogEntry — một giai đoạn lớn (bảng §5.2).
type E2EStageCatalogEntry struct {
	ID            string `json:"id"`                    // G1…G6
	SwimlaneCode  string `json:"swimlaneCode"`          // Mã lưu đồ §1.1, có thể ghép "ING,DOM"
	TitleVi       string `json:"titleVi"`               // Tên giai đoạn
	SummaryVi     string `json:"summaryVi"`             // Mô tả kỹ thuật / luồng máy
	UserSummaryVi string `json:"userSummaryVi"`         // Mô tả thân thiện cho người dùng cuối (timeline / onboarding)
	IngressHint   string `json:"ingressHint,omitempty"` // Pha chính (trục vụ)
}

// E2EStepCatalogEntry — một dòng bảng chi tiết §5.3 (bước + sự kiện + queue fields).
type E2EStepCatalogEntry struct {
	StageID                string `json:"stageId"`                       // G1…
	StepID                 string `json:"stepId"`                        // Gx-Syy
	EventDetailID          string `json:"eventDetailId,omitempty"`       // Gx-Syy-Ezz khi có
	DescriptionTechnicalVi string `json:"descriptionTechnicalVi"`        // Mô tả kỹ thuật (tra code, audit)
	DescriptionUserVi      string `json:"descriptionUserVi"`             // Mô tả thân thiện end-user (UI tooltip)
	EventType              string `json:"eventType,omitempty"`           // — → rỗng
	EventSource            string `json:"eventSource,omitempty"`         //
	PipelineStage          string `json:"pipelineStage,omitempty"`       //
	ResponsibilityGroup    string `json:"responsibilityGroup,omitempty"` // Nhóm trách nhiệm / swimlane tổng quát
}

// E2EStageCatalog — bảng giai đoạn lớn §5.2 (bản chữ trong code; đổi doc thì sửa đồng bộ).
func E2EStageCatalog() []E2EStageCatalogEntry {
	return []E2EStageCatalogEntry{
		{ID: E2EStageG1, SwimlaneCode: "ING,DOM", TitleVi: "Pha ghi thô: CIO ingress → L1 → datachanged → enqueue", SummaryVi: "CIO/sync → L1 → EmitDataChanged → decision_events_queue (eventSource l1_datachanged)", UserSummaryVi: "Dữ liệu từ cửa hàng và kênh chat được thu về, lưu nhất quán; hệ thống ghi nhận thay đổi và chuẩn bị đưa vào hàng đợi xử lý của trợ lý.", IngressHint: "Pha ghi thô"},
		{ID: E2EStageG2, SwimlaneCode: "AID,DOM", TitleVi: "Pha merge: consumer queue + hợp nhất canonical (L2)", SummaryVi: "Hai tầng: (1) Một job decision_events_queue — lease → applyDatachangedSideEffects (chỉ L1 datachanged) → dispatchConsumerEvent. (2) Worker miền — merge L1→L2 canonical → có thể crm.intelligence.recompute_requested.", UserSummaryVi: "Trước: máy nhận việc trong hàng đợi trợ lý, chạy tác vụ phụ và handler. Sau (queue/worker khác): gộp hồ sơ canonical, rồi có thể yêu cầu tính lại chỉ số khách.", IngressHint: "Pha merge"},
		{ID: E2EStageG3, SwimlaneCode: "INT", TitleVi: "Pha intel miền và bàn giao về AID", SummaryVi: "Job *_intel_compute → ghi kết quả → *_intel_recomputed", UserSummaryVi: "Phân tích chuyên sâu theo miền đã cấu hình; kết quả chuyển về trợ lý để ra quyết định.", IngressHint: "Pha intel"},
		{ID: E2EStageG4, SwimlaneCode: "AID", TitleVi: "Pha ra quyết định: case, ngữ cảnh, điều phối", SummaryVi: "decision_cases_runtime, *.context_*, aidecision.execute_requested / executor.propose_requested", UserSummaryVi: "Trợ lý gom ngữ cảnh, có thể gom tin nhắn theo cửa sổ thời gian, áp quy tắc rồi đề xuất hoặc chuẩn bị hành động phù hợp tình huống.", IngressHint: "Pha ra quyết định"},
		{ID: E2EStageG5, SwimlaneCode: "EXC", TitleVi: "Pha thực thi (Executor)", SummaryVi: "Duyệt, dispatch adapter, chạy action", UserSummaryVi: "Lệnh được duyệt (tự động hoặc có người) và gửi tới hệ thống thực thi để hoàn tất việc cần làm.", IngressHint: "Pha thực thi"},
		{ID: E2EStageG6, SwimlaneCode: "OUT,LRN,FBK", TitleVi: "Pha học: outcome, learning_cases, feedback", SummaryVi: "Outcome → learning_cases → evaluation → gợi ý rule/policy", UserSummaryVi: "Kết quả thực tế được ghi nhận; hệ thống học từ phản hồi để gợi ý cải tiến quy tắc và trải nghiệm sau này.", IngressHint: "Pha học"},
	}
}

// E2EStepCatalog — bảng chi tiết §5.3 (một bảng thống nhất).
func E2EStepCatalog() []E2EStepCatalogEntry {
	return []E2EStepCatalogEntry{
		{StageID: E2EStageG1, StepID: "G1-S01", DescriptionTechnicalVi: "CIO / kênh ingress nhận dữ liệu từ nguồn bên ngoài (webhook, job sync kênh, callback…); xác thực & ghi nhận — chưa ghi L1", DescriptionUserVi: "Cửa hàng nhận tin từ khách qua các kênh (webhook, đồng bộ…); mới kiểm tra và ghi nhận, chưa lưu vào kho dữ liệu nội bộ.", ResponsibilityGroup: "CIO"},
		{StageID: E2EStageG1, StepID: "G1-S02", DescriptionTechnicalVi: "CIO xử lý payload: chuẩn hoá, map schema; ghi mirror / L1-persist (DoSyncUpsert) hoặc CRUD domain", DescriptionUserVi: "Dữ liệu nghiệp vụ được chuẩn hoá và lưu bản ghi nguồn (L1) cho các bước sau.", ResponsibilityGroup: "CIO"},
		{StageID: E2EStageG1, StepID: "G1-S03", DescriptionTechnicalVi: "Sau khi ghi L1 thành công: phát EmitDataChanged (bus nội bộ). Handler AID (hooks/datachanged) lọc collection: source_sync registry + ShouldEmitDatachangedToDecisionQueue — chỉ hợp lệ mới enqueue G1-S04", DescriptionUserVi: "Sau khi lưu xong, hệ thống báo nội bộ có thay đổi; chỉ những loại dữ liệu đã cấu hình mới được chuyển tiếp tới hàng đợi trợ lý.", ResponsibilityGroup: "CIO"},
		{StageID: E2EStageG1, StepID: "G1-S04", EventDetailID: "G1-S04-E01", DescriptionTechnicalVi: "Enqueue decision_events_queue từ hook datachanged sau L1; chi tiết wire theo eventType.", DescriptionUserVi: "Cập nhật nguồn được đưa vào hàng đợi trợ lý; loại thực thể xem cột eventType.", EventType: "conversation.changed", EventSource: "l1_datachanged", PipelineStage: "after_l1_change", ResponsibilityGroup: "CIO"},
		{StageID: E2EStageG1, StepID: "G1-S04", EventDetailID: "G1-S04-E02", DescriptionTechnicalVi: "Enqueue decision_events_queue từ hook datachanged sau L1; chi tiết wire theo eventType.", DescriptionUserVi: "Cập nhật nguồn được đưa vào hàng đợi trợ lý; loại thực thể xem cột eventType.", EventType: "message.changed", EventSource: "l1_datachanged", PipelineStage: "after_l1_change", ResponsibilityGroup: "CIO"},
		{StageID: E2EStageG1, StepID: "G1-S04", EventDetailID: "G1-S04-E03", DescriptionTechnicalVi: "Enqueue decision_events_queue từ hook datachanged sau L1; chi tiết wire theo eventType.", DescriptionUserVi: "Cập nhật nguồn được đưa vào hàng đợi trợ lý; loại thực thể xem cột eventType.", EventType: "order.changed", EventSource: "l1_datachanged", PipelineStage: "after_l1_change", ResponsibilityGroup: "CIO"},
		{StageID: E2EStageG1, StepID: "G1-S04", EventDetailID: "G1-S04-E04", DescriptionTechnicalVi: "Enqueue decision_events_queue từ hook datachanged sau L1; chi tiết wire theo eventType.", DescriptionUserVi: "Cập nhật nguồn được đưa vào hàng đợi trợ lý; loại thực thể xem cột eventType.", EventType: "meta_ad_insight.updated", EventSource: "l1_datachanged", PipelineStage: "after_l1_change", ResponsibilityGroup: "CIO"},
		{StageID: E2EStageG1, StepID: "G1-S04", EventDetailID: "G1-S04-E05", DescriptionTechnicalVi: "Enqueue decision_events_queue từ hook datachanged sau L1; chi tiết wire theo eventType.", DescriptionUserVi: "Cập nhật nguồn được đưa vào hàng đợi trợ lý; loại thực thể xem cột eventType.", EventType: "pos_customer.* / fb_customer.* / crm_customer.updated", EventSource: "l1_datachanged", PipelineStage: "after_l1_change", ResponsibilityGroup: "CIO"},
		{StageID: E2EStageG2, StepID: "G2-S01", DescriptionTechnicalVi: "AIDecisionConsumerWorker lease một bản ghi decision_events_queue — bắt đầu processEvent.", DescriptionUserVi: "Máy lấy một việc đã xếp hàng để xử lý.", ResponsibilityGroup: "AID"},
		{StageID: E2EStageG2, StepID: "G2-S02", DescriptionTechnicalVi: "Chỉ khi IsL1DatachangedEventSource: applyDatachangedSideEffects — hydrate, đọc Mongo, datachangedsidefx.Run (side-effect đã đăng ký: merge chờ, báo cáo, debounce intel…). Event khác: bỏ qua.", DescriptionUserVi: "Với thay đổi từ L1, chạy tác vụ phụ đã đăng ký (có thể trì hoãn); loại job khác không qua bước này.", ResponsibilityGroup: "AID"},
		{StageID: E2EStageG2, StepID: "G2-S03", DescriptionTechnicalVi: "dispatchConsumerEvent: consumerreg.Lookup(eventType) → handler; không có handler → no_handler (không lỗi).", DescriptionUserVi: "Gọi handler đã đăng ký theo eventType (intel, case, điều phối…).", ResponsibilityGroup: "AID"},
		{StageID: E2EStageG2, StepID: "G2-S04", DescriptionTechnicalVi: "Worker miền lấy job gộp L1→L2 — tách khỏi consumer S01–S03 (queue/worker theo từng miền dữ liệu).", DescriptionUserVi: "Luồng worker riêng nhận việc gộp dữ liệu từ nhiều nguồn.", ResponsibilityGroup: "DomainData"},
		{StageID: E2EStageG2, StepID: "G2-S05", DescriptionTechnicalVi: "Áp merge: ghi canonical (uid, sourceIds, links).", DescriptionUserVi: "Cập nhật hồ sơ chung để mọi kênh trỏ cùng một thực thể.", ResponsibilityGroup: "DomainData"},
		{StageID: E2EStageG2, StepID: "G2-S06", EventDetailID: "G2-S06-E01", DescriptionTechnicalVi: "Sau merge L2: emit yêu cầu tính lại intel (minh hoạ CRM: crm.intelligence.recompute_requested, crm_merge_queue, after_l2_merge).", DescriptionUserVi: "Sau khi gộp canonical, có thể kích hoạt làm mới phân tích theo miền (chi tiết ở eventType).", EventType: "crm.intelligence.recompute_requested", EventSource: "crm_merge_queue", PipelineStage: "after_l2_merge", ResponsibilityGroup: "DomainData"},
		{StageID: E2EStageG3, StepID: "G3-S01", EventDetailID: "G3-S01-E01", DescriptionTechnicalVi: "Yêu cầu intel/recompute — miền, nguồn và pipeline theo eventType và các cột kế bên.", DescriptionUserVi: "Kích hoạt hoặc xếp hàng phân tích chuyên sâu theo miền tương ứng.", EventType: "crm.intelligence.compute_requested", EventSource: "crm", PipelineStage: "after_l1_change", ResponsibilityGroup: "DomainIntel"},
		{StageID: E2EStageG3, StepID: "G3-S01", EventDetailID: "G3-S01-E02", DescriptionTechnicalVi: "Yêu cầu intel/recompute — miền, nguồn và pipeline theo eventType và các cột kế bên.", DescriptionUserVi: "Kích hoạt hoặc xếp hàng phân tích chuyên sâu theo miền tương ứng.", EventType: "crm.intelligence.recompute_requested", EventSource: "crm hoặc crm_merge_queue", PipelineStage: "after_l1_change hoặc after_l2_merge", ResponsibilityGroup: "DomainIntel"},
		{StageID: E2EStageG3, StepID: "G3-S01", EventDetailID: "G3-S01-E03", DescriptionTechnicalVi: "Yêu cầu intel/recompute — miền, nguồn và pipeline theo eventType và các cột kế bên.", DescriptionUserVi: "Kích hoạt hoặc xếp hàng phân tích chuyên sâu theo miền tương ứng.", EventType: "ads.intelligence.recompute_requested", EventSource: "meta_hooks hoặc meta_api", PipelineStage: "after_l1_change hoặc external_ingest", ResponsibilityGroup: "DomainIntel"},
		{StageID: E2EStageG3, StepID: "G3-S01", EventDetailID: "G3-S01-E04", DescriptionTechnicalVi: "Yêu cầu intel/recompute — miền, nguồn và pipeline theo eventType và các cột kế bên.", DescriptionUserVi: "Kích hoạt hoặc xếp hàng phân tích chuyên sâu theo miền tương ứng.", EventType: "order.recompute_requested", EventSource: "tuỳ đường emit", PipelineStage: "tuỳ đường emit", ResponsibilityGroup: "DomainIntel"},
		{StageID: E2EStageG3, StepID: "G3-S01", EventDetailID: "G3-S01-E05", DescriptionTechnicalVi: "Yêu cầu intel/recompute — miền, nguồn và pipeline theo eventType và các cột kế bên.", DescriptionUserVi: "Kích hoạt hoặc xếp hàng phân tích chuyên sâu theo miền tương ứng.", EventType: "cix.analysis_requested", EventSource: "aidecision hoặc cix_api", PipelineStage: "aid_coordination hoặc external_ingest", ResponsibilityGroup: "DomainIntel"},
		{StageID: E2EStageG3, StepID: "G3-S01", EventDetailID: "G3-S01-E06", DescriptionTechnicalVi: "Order intelligence (legacy trong queue cũ)", DescriptionUserVi: "Luồng phân tích đơn hàng kiểu cũ (tương thích bản ghi trước đây).", EventType: "order.intelligence_requested (legacy)", EventSource: "tuỳ bản ghi", PipelineStage: "tuỳ bản ghi", ResponsibilityGroup: "DomainIntel"},
		{StageID: E2EStageG3, StepID: "G3-S02", DescriptionTechnicalVi: "Worker miền lấy job *_intel_compute", DescriptionUserVi: "Máy chủ bắt đầu chạy bài phân tích đã xếp hàng.", ResponsibilityGroup: "DomainIntel"},
		{StageID: E2EStageG3, StepID: "G3-S03", DescriptionTechnicalVi: "Chạy pipeline phân tích (rule/LLM/snapshot)", DescriptionUserVi: "Áp quy tắc và mô hình để tóm tắt, đánh giá hoặc gắn nhãn theo cấu hình.", ResponsibilityGroup: "DomainIntel"},
		{StageID: E2EStageG3, StepID: "G3-S04", DescriptionTechnicalVi: "Ghi bản ghi chạy intel + cập nhật read model", DescriptionUserVi: "Kết quả phân tích được lưu để màn hình và trợ lý đọc nhanh, không phải tính lại mỗi lần.", ResponsibilityGroup: "DomainIntel"},
		{StageID: E2EStageG3, StepID: "G3-S05", EventDetailID: "G3-S05-E01", DescriptionTechnicalVi: "Worker miền emit *_intel_recomputed — bàn giao kết quả phân tích về AID (miền theo eventType).", DescriptionUserVi: "Kết quả phân tích được chuyển về trợ lý.", EventType: "cix_intel_recomputed", EventSource: "cix_intel", PipelineStage: "domain_intel", ResponsibilityGroup: "DomainIntel"},
		{StageID: E2EStageG3, StepID: "G3-S05", EventDetailID: "G3-S05-E02", DescriptionTechnicalVi: "Worker miền emit *_intel_recomputed — bàn giao kết quả phân tích về AID (miền theo eventType).", DescriptionUserVi: "Kết quả phân tích được chuyển về trợ lý.", EventType: "crm_intel_recomputed", EventSource: "crm_intel", PipelineStage: "domain_intel", ResponsibilityGroup: "DomainIntel"},
		{StageID: E2EStageG3, StepID: "G3-S05", EventDetailID: "G3-S05-E03", DescriptionTechnicalVi: "Worker miền emit *_intel_recomputed — bàn giao kết quả phân tích về AID (miền theo eventType).", DescriptionUserVi: "Kết quả phân tích được chuyển về trợ lý.", EventType: "order_intel_recomputed", EventSource: "order_intel", PipelineStage: "domain_intel", ResponsibilityGroup: "DomainIntel"},
		{StageID: E2EStageG3, StepID: "G3-S05", EventDetailID: "G3-S05-E04", DescriptionTechnicalVi: "Worker miền emit *_intel_recomputed — bàn giao kết quả phân tích về AID (miền theo eventType).", DescriptionUserVi: "Kết quả phân tích được chuyển về trợ lý.", EventType: "campaign_intel_recomputed", EventSource: "meta_ads_intel", PipelineStage: "domain_intel", ResponsibilityGroup: "DomainIntel"},
		{StageID: E2EStageG4, StepID: "G4-S01", DescriptionTechnicalVi: "ResolveOrCreate và cập nhật decision_cases_runtime", DescriptionUserVi: "Trợ lý mở hoặc cập nhật một «vụ việc» để theo dõi xử lý từ đầu đến cuối.", ResponsibilityGroup: "AID"},
		{StageID: E2EStageG4, StepID: "G4-S02", EventDetailID: "G4-S02-E01", DescriptionTechnicalVi: "Yêu cầu ngữ cảnh bổ sung — loại theo eventType.", DescriptionUserVi: "Trợ lý xin thêm dữ liệu nền trước khi quyết định.", EventType: "customer.context_requested", EventSource: "aidecision", PipelineStage: "aid_coordination", ResponsibilityGroup: "AID"},
		{StageID: E2EStageG4, StepID: "G4-S02", EventDetailID: "G4-S02-E02", DescriptionTechnicalVi: "Ngữ cảnh đã sẵn sàng — loại theo eventType.", DescriptionUserVi: "Đủ dữ liệu nền để trợ lý tiếp tục.", EventType: "customer.context_ready", EventSource: "crm / aidecision", PipelineStage: "aid_coordination", ResponsibilityGroup: "AID"},
		{StageID: E2EStageG4, StepID: "G4-S02", EventDetailID: "G4-S02-E03", DescriptionTechnicalVi: "Yêu cầu ngữ cảnh bổ sung — loại theo eventType.", DescriptionUserVi: "Trợ lý xin thêm dữ liệu nền trước khi quyết định.", EventType: "ads.context_requested", EventSource: "aidecision", PipelineStage: "aid_coordination", ResponsibilityGroup: "AID"},
		{StageID: E2EStageG4, StepID: "G4-S02", EventDetailID: "G4-S02-E04", DescriptionTechnicalVi: "Ngữ cảnh đã sẵn sàng — loại theo eventType.", DescriptionUserVi: "Đủ dữ liệu nền để trợ lý tiếp tục.", EventType: "ads.context_ready", EventSource: "meta_ads_intel / aidecision", PipelineStage: "aid_coordination", ResponsibilityGroup: "AID"},
		{StageID: E2EStageG4, StepID: "G4-S03", EventDetailID: "G4-S03-E01", DescriptionTechnicalVi: "Flush debounce cho tin nhắn", DescriptionUserVi: "Một loạt tin nhắn gần nhau được gom lại thành một lần xử lý để tránh spam và phản hồi mượt hơn.", EventType: "message.batch_ready", EventSource: "debounce", PipelineStage: "aid_coordination", ResponsibilityGroup: "AID"},
		{StageID: E2EStageG4, StepID: "G4-S04", DescriptionTechnicalVi: "Áp policy + kiểm HasAllRequiredContexts + chọn nhánh", DescriptionUserVi: "Trợ lý kiểm tra đủ thông tin và quy tắc, rồi chọn hướng xử lý phù hợp.", ResponsibilityGroup: "AID"},
		{StageID: E2EStageG4, StepID: "G4-S05", EventDetailID: "G4-S05-E01", DescriptionTechnicalVi: "Phát lệnh thực thi", DescriptionUserVi: "Trợ lý chuẩn bị gửi hành động đi thực hiện (gửi tin, cập nhật hệ thống…).", EventType: "aidecision.execute_requested", EventSource: "aidecision", PipelineStage: "aid_coordination", ResponsibilityGroup: "AID"},
		{StageID: E2EStageG4, StepID: "G4-S05", EventDetailID: "G4-S05-E02", DescriptionTechnicalVi: "Phát yêu cầu đề xuất vào Executor", DescriptionUserVi: "Trợ lý tạo đề xuất để bạn hoặc hệ thống duyệt trước khi chạy.", EventType: "executor.propose_requested", EventSource: "aidecision", PipelineStage: "aid_coordination", ResponsibilityGroup: "AID"},
		{StageID: E2EStageG4, StepID: "G4-S05", EventDetailID: "G4-S05-E03", DescriptionTechnicalVi: "Event cũ tương thích cho Ads", DescriptionUserVi: "Luồng đề xuất quảng cáo kiểu cũ (tương thích hệ thống trước).", EventType: "ads.propose_requested (legacy)", EventSource: "aidecision", PipelineStage: "aid_coordination", ResponsibilityGroup: "AID"},
		{StageID: E2EStageG5, StepID: "G5-S01", DescriptionTechnicalVi: "Executor nhận proposal/action từ AI Decision", DescriptionUserVi: "Khối thực thi nhận lệnh hoặc đề xuất từ trợ lý.", ResponsibilityGroup: "Executor"},
		{StageID: E2EStageG5, StepID: "G5-S02", DescriptionTechnicalVi: "Duyệt theo policy (manual/auto/...)", DescriptionUserVi: "Hành động được duyệt tự động hoặc chờ người xác nhận tùy cài đặt.", ResponsibilityGroup: "Executor"},
		{StageID: E2EStageG5, StepID: "G5-S03", DescriptionTechnicalVi: "Dispatch adapter để thực thi", DescriptionUserVi: "Lệnh được gửi đúng kênh kỹ thuật (API, tin nhắn…) để hoàn tất.", ResponsibilityGroup: "Executor"},
		{StageID: E2EStageG6, StepID: "G6-S01", DescriptionTechnicalVi: "Ghi kết quả kỹ thuật (delivery/API response)", DescriptionUserVi: "Hệ thống ghi nhận đã gửi thành công hay lỗi kỹ thuật.", ResponsibilityGroup: "Outcome"},
		{StageID: E2EStageG6, StepID: "G6-S02", DescriptionTechnicalVi: "Thu kết quả nghiệp vụ theo time window", DescriptionUserVi: "Theo dõi kết quả thực tế sau một khoảng thời gian (theo cấu hình vụ việc).", ResponsibilityGroup: "Outcome"},
		{StageID: E2EStageG6, StepID: "G6-S03", DescriptionTechnicalVi: "Action chuyển trạng thái kết thúc (executed/rejected/failed)", DescriptionUserVi: "Việc được đánh dấu hoàn thành, từ chối hoặc thất bại rõ ràng.", ResponsibilityGroup: "Learning"},
		{StageID: E2EStageG6, StepID: "G6-S04", DescriptionTechnicalVi: "OnActionClosed → CreateLearningCaseFromAction → insert learning_cases", DescriptionUserVi: "Từ kết thúc việc, hệ thống tạo bản ghi học để đánh giá sau.", ResponsibilityGroup: "Learning"},
		{StageID: E2EStageG6, StepID: "G6-S05", DescriptionTechnicalVi: "Chạy RunEvaluationBatch / job đánh giá", DescriptionUserVi: "Chạy đợt đánh giá chất lượng gợi ý và kết quả.", ResponsibilityGroup: "Learning"},
		{StageID: E2EStageG6, StepID: "G6-S06", DescriptionTechnicalVi: "Ghi field evaluation (ví dụ outcome_class, attribution)", DescriptionUserVi: "Gắn nhãn kết quả (tốt/xấu, nguyên nhân) phục vụ báo cáo.", ResponsibilityGroup: "Learning"},
		{StageID: E2EStageG6, StepID: "G6-S07", DescriptionTechnicalVi: "Sinh param_suggestions, rule_candidate, insight", DescriptionUserVi: "Sinh gợi ý chỉnh tham số hoặc quy tắc dựa trên dữ liệu thực tế.", ResponsibilityGroup: "Feedback"},
		{StageID: E2EStageG6, StepID: "G6-S08", DescriptionTechnicalVi: "Đẩy ngược cải tiến lên Rule/Policy/AID (có thể có bước duyệt người)", DescriptionUserVi: "Gợi ý cải tiến được đưa lên bảng điều khiển quy tắc (có thể cần người duyệt).", ResponsibilityGroup: "Feedback"},
	}
}

// E2EQueueMilestoneCatalogEntry — mốc consumer (timeline) khớp ResolveE2EForQueueConsumerMilestone.
type E2EQueueMilestoneCatalogEntry struct {
	Key         string `json:"key"`     // processing_start | …
	StageID     string `json:"stageId"` // G2 (pha merge — consumer)
	StepID      string `json:"stepId"`
	LabelVi     string `json:"labelVi"`     // Mô tả kỹ thuật / máy
	UserLabelVi string `json:"userLabelVi"` // Mô tả thân thiện end-user
}

// E2EQueueMilestoneCatalog — các milestone nội bộ consumer (eventtypes.E2EQueueMilestone*).
func E2EQueueMilestoneCatalog() []E2EQueueMilestoneCatalogEntry {
	return []E2EQueueMilestoneCatalogEntry{
		{Key: E2EQueueMilestoneProcessingStart, StageID: E2EStageG2, StepID: "G2-S01", LabelVi: "Consumer nhận job — bắt đầu xử lý", UserLabelVi: "Hệ thống bắt đầu xử lý việc đã xếp hàng."},
		{Key: E2EQueueMilestoneDatachangedDone, StageID: E2EStageG2, StepID: "G2-S02", LabelVi: "Hoàn tất tác vụ sau datachanged (consumer)", UserLabelVi: "Đã xong các bước đồng bộ sau khi dữ liệu thay đổi."},
		{Key: E2EQueueMilestoneHandlerDone, StageID: E2EStageG2, StepID: "G2-S03", LabelVi: "Đóng job consumer — handler hoàn tất", UserLabelVi: "Đã xử lý xong việc này trên hàng đợi."},
		{Key: E2EQueueMilestoneHandlerError, StageID: E2EStageG2, StepID: "G2-S03", LabelVi: "Lỗi xử lý trên consumer", UserLabelVi: "Có lỗi khi xử lý; có thể thử lại hoặc xem chi tiết lỗi."},
		{Key: E2EQueueMilestoneRoutingSkipped, StageID: E2EStageG2, StepID: "G2-S03", LabelVi: "Routing bỏ qua handler (noop)", UserLabelVi: "Theo cài đặt, bước tự động này được bỏ qua — không ảnh hưởng dữ liệu đã lưu."},
		{Key: E2EQueueMilestoneNoHandler, StageID: E2EStageG2, StepID: "G2-S03", LabelVi: "Chưa có handler đăng ký cho eventType", UserLabelVi: "Loại cập nhật này chưa được cấu hình xử lý tự động thêm."},
	}
}
