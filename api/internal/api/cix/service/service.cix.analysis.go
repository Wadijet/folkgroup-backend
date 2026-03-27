// Package service — Service cho module CIX (Contextual Conversation Intelligence).
//
// CixAnalysisService xử lý phân tích hội thoại — Raw → L1 → L2 → L3 → Flag → Action.
// Đọc conversation từ fb_message_items, customer context từ CRM, chạy Rule Engine pipeline.
package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	cixdto "meta_commerce/internal/api/cix/dto"
	cixmodels "meta_commerce/internal/api/cix/models"
	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	ruleintelengine "meta_commerce/internal/api/ruleintel/engine"
	ruleintelmodels "meta_commerce/internal/api/ruleintel/models"
	ruleintelsvc "meta_commerce/internal/api/ruleintel/service"

	basesvc "meta_commerce/internal/api/base/service"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CixAnalysisService service phân tích hội thoại.
type CixAnalysisService struct {
	*basesvc.BaseServiceMongoImpl[cixmodels.CixAnalysisResult]
}

// NewCixAnalysisService tạo service mới.
func NewCixAnalysisService() (*CixAnalysisService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CixAnalysisResults)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.CixAnalysisResults, common.ErrNotFound)
	}
	return &CixAnalysisService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[cixmodels.CixAnalysisResult](coll),
	}, nil
}

// getConversationTurns đọc transcript từ fb_message_items theo conversationId.
func (s *CixAnalysisService) getConversationTurns(ctx context.Context, conversationId string, ownerOrgID primitive.ObjectID) ([]map[string]interface{}, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbMessageItems)
	if !ok {
		return nil, nil
	}
	filter := bson.M{"conversationId": conversationId, "ownerOrganizationId": ownerOrgID}
	opts := options.Find().SetSort(bson.D{{Key: "insertedAt", Value: 1}}).SetLimit(500)
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var items []struct {
		MessageData map[string]interface{} `bson:"messageData"`
		InsertedAt  int64                  `bson:"insertedAt"`
	}
	if err = cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	turns := make([]map[string]interface{}, 0, len(items))
	for _, it := range items {
		from := "customer"
		if v, ok := it.MessageData["message"].(map[string]interface{}); ok {
			if dir, _ := v["direction"].(string); dir == "out" {
				from = "agent"
			}
		}
		text := ""
		if v, ok := it.MessageData["message"].(map[string]interface{}); ok {
			if t, _ := v["text"].(string); t != "" {
				text = t
			}
		}
		turns = append(turns, map[string]interface{}{
			"from":      from,
			"content":   text,
			"timestamp": it.InsertedAt,
		})
	}
	return turns, nil
}

// getCustomerContext lấy context khách từ CRM (valueTier, journeyStage, lifecycleStage).
func (s *CixAnalysisService) getCustomerContext(ctx context.Context, customerIdOrUid string, ownerOrgID primitive.ObjectID) map[string]interface{} {
	if customerIdOrUid == "" {
		return map[string]interface{}{}
	}
	crmSvc, err := crmvc.NewCrmCustomerService()
	if err != nil {
		return map[string]interface{}{}
	}
	profile, err := crmSvc.GetProfile(ctx, customerIdOrUid, ownerOrgID)
	if err != nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"valueTier":      profile.ValueTier,
		"lifecycleStage": profile.LifecycleStage,
		"journeyStage":   profile.JourneyStage,
	}
}

// appendCixPipelineTrace thêm trace_id từ một lần Run (bỏ qua trùng liên tiếp).
func appendCixPipelineTrace(dst *[]string, runRes *ruleintelengine.RunResult) {
	if dst == nil || runRes == nil {
		return
	}
	t := strings.TrimSpace(runRes.TraceID)
	if t == "" {
		return
	}
	if len(*dst) > 0 && (*dst)[len(*dst)-1] == t {
		return
	}
	*dst = append(*dst, t)
}

// runPipeline chạy Rule Engine pipeline Raw → L1 → L2 → L3 → Flag → Action.
func (s *CixAnalysisService) runPipeline(ctx context.Context, raw map[string]interface{}, customerCtx map[string]interface{}, ownerOrgID primitive.ObjectID) (*cixmodels.CixAnalysisResult, error) {
	ruleSvc, err := ruleintelsvc.NewRuleEngineService()
	if err != nil {
		return nil, err
	}
	entityRef := ruleintelmodels.EntityRef{
		Domain:              "cix",
		ObjectType:          "conversation",
		ObjectID:            "",
		OwnerOrganizationID: ownerOrgID.Hex(),
	}
	layers := map[string]interface{}{
		"cix_raw":             raw,
		"cix_customer_context": customerCtx,
	}

	var pipelineTraces []string

	// L1
	runRes, err := ruleSvc.Run(ctx, &ruleintelsvc.RunInput{RuleID: "RULE_CIX_LAYER1_STAGE", Domain: "cix", EntityRef: entityRef, Layers: layers})
	if err != nil {
		return nil, err
	}
	appendCixPipelineTrace(&pipelineTraces, runRes)
	if out, ok := runRes.Result.(map[string]interface{}); ok {
		layers["cix_layer1"] = out
	}

	// L2
	runRes, err = ruleSvc.Run(ctx, &ruleintelsvc.RunInput{RuleID: "RULE_CIX_LAYER2_STATE", Domain: "cix", EntityRef: entityRef, Layers: layers})
	if err != nil {
		return nil, err
	}
	appendCixPipelineTrace(&pipelineTraces, runRes)
	if out, ok := runRes.Result.(map[string]interface{}); ok {
		layers["cix_layer2"] = out
	}

	// L2 Adj
	runRes, err = ruleSvc.Run(ctx, &ruleintelsvc.RunInput{RuleID: "RULE_CIX_LAYER2_ADJUST", Domain: "cix", EntityRef: entityRef, Layers: layers})
	if err != nil {
		return nil, err
	}
	appendCixPipelineTrace(&pipelineTraces, runRes)
	if out, ok := runRes.Result.(map[string]interface{}); ok {
		layers["cix_layer2_adj"] = out
	}

	// L3 — Rule hoặc LLM (theo CIX_LAYER3_MODE: rule | llm | hybrid)
	layer3, l3Trace := s.resolveLayer3(ctx, raw, customerCtx, ownerOrgID, ruleSvc, entityRef, layers)
	if l3Trace != "" {
		appendCixPipelineTrace(&pipelineTraces, &ruleintelengine.RunResult{TraceID: l3Trace})
	}
	layers["cix_layer3"] = map[string]interface{}{
		"buyingIntent":   layer3.BuyingIntent,
		"objectionLevel": layer3.ObjectionLevel,
		"sentiment":      layer3.Sentiment,
	}

	// Flags
	runRes, err = ruleSvc.Run(ctx, &ruleintelsvc.RunInput{RuleID: "RULE_CIX_FLAGS", Domain: "cix", EntityRef: entityRef, Layers: layers})
	if err != nil {
		return nil, err
	}
	appendCixPipelineTrace(&pipelineTraces, runRes)
	if out, ok := runRes.Result.(map[string]interface{}); ok {
		layers["cix_flags"] = out
	}

	// Actions — traceId từ lần chạy này dùng làm neo tới rule_execution_logs (đề xuất hành động).
	runRes, err = ruleSvc.Run(ctx, &ruleintelsvc.RunInput{RuleID: "RULE_CIX_ACTIONS", Domain: "cix", EntityRef: entityRef, Layers: layers})
	if err != nil {
		return nil, err
	}
	appendCixPipelineTrace(&pipelineTraces, runRes)
	cixRuleTraceID := strings.TrimSpace(runRes.TraceID)

	// Build result
	L1 := layers["cix_layer1"].(map[string]interface{})
	L2 := layers["cix_layer2"].(map[string]interface{})
	L2Adj := layers["cix_layer2_adj"].(map[string]interface{})
	L3 := layers["cix_layer3"].(map[string]interface{})
	flags := layers["cix_flags"].(map[string]interface{})
	actions := runRes.Result.(map[string]interface{})

	var flagsList []cixmodels.CixFlag
	if farr, ok := flags["flags"].([]interface{}); ok {
		for _, f := range farr {
			if m, ok := f.(map[string]interface{}); ok {
				flagsList = append(flagsList, cixmodels.CixFlag{
					Name:           getStr(m, "name"),
					Severity:       getStr(m, "severity"),
					TriggeredByRule: getStr(m, "triggeredByRule"),
				})
			}
		}
	}
	var actionList []string
	if arr, ok := actions["actionSuggestions"].([]interface{}); ok {
		for _, a := range arr {
			if str, ok := a.(string); ok && str != "none" {
				actionList = append(actionList, str)
			}
		}
		if len(actionList) == 0 {
			actionList = []string{}
		}
	}

	return &cixmodels.CixAnalysisResult{
		TraceID:              cixRuleTraceID,
		PipelineRuleTraceIDs: pipelineTraces,
		Layer1:               cixmodels.CixLayer1{Stage: getStr(L1, "stage")},
		Layer2: cixmodels.CixLayer2{
			IntentStage:      getStr(L2, "intentStage"),
			UrgencyLevel:     getStr(L2, "urgencyLevel"),
			RiskLevelRaw:     getStr(L2, "riskLevelRaw"),
			RiskLevelAdj:     getStr(L2Adj, "riskLevelAdj"),
			AdjustmentRule:   getStr(L2Adj, "ruleId"),
			AdjustmentReason: getStr(L2Adj, "adjustmentReason"),
		},
		Layer3: cixmodels.CixLayer3{
			BuyingIntent:   getStr(L3, "buyingIntent"),
			ObjectionLevel: getStr(L3, "objectionLevel"),
			Sentiment:      getStr(L3, "sentiment"),
		},
		Flags:             flagsList,
		ActionSuggestions: actionList,
	}, nil
}

// getCixLayer3Mode trả mode Layer 3 từ env: rule | llm | hybrid. Mặc định: rule.
func getCixLayer3Mode() string {
	m := strings.TrimSpace(strings.ToLower(os.Getenv("CIX_LAYER3_MODE")))
	switch m {
	case "rule", "llm", "hybrid":
		return m
	default:
		return "rule"
	}
}

// resolveLayer3 quyết định Layer 3 từ Rule hoặc LLM theo CIX_LAYER3_MODE. Chuỗi thứ hai là trace_id RULE_CIX_LAYER3_SIGNALS khi có chạy rule.
func (s *CixAnalysisService) resolveLayer3(ctx context.Context, raw map[string]interface{}, customerCtx map[string]interface{}, ownerOrgID primitive.ObjectID, ruleSvc *ruleintelsvc.RuleEngineService, entityRef ruleintelmodels.EntityRef, layers map[string]interface{}) (cixmodels.CixLayer3, string) {
	mode := getCixLayer3Mode()
	turns, _ := raw["turns"].([]map[string]interface{})

	runRuleL3 := func() (cixmodels.CixLayer3, string) {
		runRes, err := ruleSvc.Run(ctx, &ruleintelsvc.RunInput{RuleID: "RULE_CIX_LAYER3_SIGNALS", Domain: "cix", EntityRef: entityRef, Layers: layers})
		if err != nil {
			return cixmodels.CixLayer3{BuyingIntent: "none", ObjectionLevel: "none", Sentiment: "neutral"}, ""
		}
		out, ok := runRes.Result.(map[string]interface{})
		if !ok {
			return cixmodels.CixLayer3{BuyingIntent: "none", ObjectionLevel: "none", Sentiment: "neutral"}, ""
		}
		return cixmodels.CixLayer3{
			BuyingIntent:   getStr(out, "buyingIntent"),
			ObjectionLevel: getStr(out, "objectionLevel"),
			Sentiment:      getStr(out, "sentiment"),
		}, strings.TrimSpace(runRes.TraceID)
	}

	tryLLM := func() *cixmodels.CixLayer3 {
		// Ưu tiên: AI provider profile từ DB (org hoặc system) → fallback env
		llmSvc := NewCixLLMServiceFromProfile(ctx, ownerOrgID)
		if llmSvc == nil {
			llmSvc = NewCixLLMService()
		}
		if !llmSvc.IsAvailable() {
			return nil
		}
		l3, err := llmSvc.ExtractLayer3Signals(ctx, turns, customerCtx)
		if err != nil {
			return nil
		}
		return l3
	}

	// rule: chỉ dùng Rule
	if mode == "rule" {
		return runRuleL3()
	}

	// llm: ưu tiên LLM, fallback Rule nếu LLM không khả dụng
	if mode == "llm" {
		if l3 := tryLLM(); l3 != nil {
			return *l3, ""
		}
		return runRuleL3()
	}

	// hybrid: Rule trước, nếu Rule trả giá trị mặc định (inquiring, neutral, none) → thử LLM
	ruleL3, l3tid := runRuleL3()
	if ruleL3.BuyingIntent != "inquiring" || ruleL3.Sentiment != "neutral" || ruleL3.ObjectionLevel != "none" {
		return ruleL3, l3tid
	}
	if l3 := tryLLM(); l3 != nil {
		// Rule đã chạy — vẫn trả trace L3 để pipeline đủ log
		return *l3, l3tid
	}
	return ruleL3, l3tid
}

func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// AnalyzeSession phân tích session — đọc conversation, customer context, chạy Rule Engine.
// sessionUid: conversationId (Messenger 1 conv = 1 session). customerUid: customerId hoặc uid.
func (s *CixAnalysisService) AnalyzeSession(ctx context.Context, sessionUid, customerUid string, ownerOrgID primitive.ObjectID) (*cixmodels.CixAnalysisResult, error) {
	now := time.Now().UnixMilli()
	conversationId := sessionUid

	turns, _ := s.getConversationTurns(ctx, conversationId, ownerOrgID)
	customerCtx := s.getCustomerContext(ctx, customerUid, ownerOrgID)

	raw := map[string]interface{}{
		"turns":       turns,
		"turnCount":   len(turns),
	}

	result, err := s.runPipeline(ctx, raw, customerCtx, ownerOrgID)
	if err != nil {
		return nil, err
	}

	result.ID = primitive.NewObjectID()
	result.OwnerOrganizationID = ownerOrgID
	result.SessionUid = sessionUid
	result.CustomerUid = customerUid
	result.CreatedAt = now

	_, err = s.InsertOne(ctx, *result)
	if err != nil {
		return nil, err
	}

	// Làm giàu profile CRM với Layer 3 signals (buyingIntent, sentiment, objectionLevel).
	if customerUid != "" {
		crmSvc, _ := crmvc.NewCrmCustomerService()
		if crmSvc != nil {
			_ = crmSvc.OnCixSignalUpdate(ctx, customerUid, ownerOrgID,
				result.Layer3.BuyingIntent, result.Layer3.Sentiment, result.Layer3.ObjectionLevel,
				result.TraceID)
		}
	}

	// Fan-in AID: InsertOne → EmitDataChanged → hook aidecision → cix_analysis_result.inserted trong decision_events_queue
	// (consumer gọi ReceiveCixPayload). Không emit thêm cix.analysis_completed ở đây để tránh trùng event.

	return result, nil
}

// FindBySessionUid tìm kết quả phân tích theo session.
func (s *CixAnalysisService) FindBySessionUid(ctx context.Context, sessionUid string, ownerOrgID primitive.ObjectID) (*cixmodels.CixAnalysisResult, error) {
	filter := bson.M{
		"sessionUid":          sessionUid,
		"ownerOrganizationId": ownerOrgID,
	}
	result, err := s.FindOne(ctx, filter, nil)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ToCixAnalysisResponse chuyển CixAnalysisResult sang DTO response.
func ToCixAnalysisResponse(r *cixmodels.CixAnalysisResult) *cixdto.CixAnalysisResponse {
	if r == nil {
		return nil
	}
	resp := &cixdto.CixAnalysisResponse{
		ID:                   r.ID.Hex(),
		SessionUid:         r.SessionUid,
		CustomerUid:          r.CustomerUid,
		TraceID:              r.TraceID,
		PipelineRuleTraceIDs: r.PipelineRuleTraceIDs,
		Layer1:               cixdto.CixLayer1DTO{Stage: r.Layer1.Stage},
		Layer2: cixdto.CixLayer2DTO{
			IntentStage:      r.Layer2.IntentStage,
			UrgencyLevel:     r.Layer2.UrgencyLevel,
			RiskLevelRaw:     r.Layer2.RiskLevelRaw,
			RiskLevelAdj:     r.Layer2.RiskLevelAdj,
			AdjustmentRule:   r.Layer2.AdjustmentRule,
			AdjustmentReason: r.Layer2.AdjustmentReason,
		},
		Layer3: cixdto.CixLayer3DTO{
			BuyingIntent:   r.Layer3.BuyingIntent,
			ObjectionLevel: r.Layer3.ObjectionLevel,
			Sentiment:      r.Layer3.Sentiment,
		},
		Flags:             make([]cixdto.CixFlagDTO, 0, len(r.Flags)),
		ActionSuggestions: r.ActionSuggestions,
		CreatedAt:         r.CreatedAt,
	}
	for _, f := range r.Flags {
		resp.Flags = append(resp.Flags, cixdto.CixFlagDTO{
			Name:            f.Name,
			Severity:        f.Severity,
			TriggeredByRule: f.TriggeredByRule,
		})
	}
	return resp
}
