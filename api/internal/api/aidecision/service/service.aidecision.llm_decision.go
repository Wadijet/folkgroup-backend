// Package aidecisionsvc — Lớp LLM cho AI Decision (Vision 08: rule-first, LLM-for-ambiguity).
//
// Bật bằng AI_DECISION_LLM_ENABLED=1. API key/model: ai_provider_profiles (OpenAI) giống CIX, fallback OPENAI_API_KEY.
package aidecisionsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	aimodels "meta_commerce/internal/api/ai/models"
	aisvc "meta_commerce/internal/api/ai/service"
	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/decisionlive/livecopy"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/cta"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

const (
	envAIDecisionLLMEnabled        = "AI_DECISION_LLM_ENABLED"
	envAIDecisionLLMRefine         = "AI_DECISION_LLM_REFINE"
	envAIDecisionLLMModel          = "AI_DECISION_LLM_MODEL"
	envAIDecisionLLMAllowedActions = "AI_DECISION_LLM_ALLOWED_ACTIONS"
	defaultAIDecisionLLMModel      = "gpt-4o-mini"
	aidecisionOpenAIProfileName    = "OpenAI Production"
)

// llmDecisionJSON cấu trúc JSON LLM trả về (khớp vision §11.1 tối giản).
type llmDecisionJSON struct {
	SelectedActions   []string `json:"selected_actions"`
	Confidence        float64  `json:"confidence"`
	ReasoningSummary  string   `json:"reasoning_summary"`
}

type llmDecisionResult struct {
	Actions          []string
	Confidence       float64
	ReasoningSummary string
	Mode             string // llm | hybrid
}

// aidecisionLLMClient client OpenAI cho tầng quyết định (không import package cix — tránh vòng phụ thuộc).
type aidecisionLLMClient struct {
	client *openai.Client
	model  string
}

func newAIDecisionLLMFromEnv() *aidecisionLLMClient {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return nil
	}
	model := strings.TrimSpace(os.Getenv(envAIDecisionLLMModel))
	if model == "" {
		model = defaultAIDecisionLLMModel
	}
	return newAIDecisionLLMWithCreds(apiKey, model, "")
}

func newAIDecisionLLMFromProfile(ctx context.Context, ownerOrgID primitive.ObjectID) *aidecisionLLMClient {
	profileSvc, err := aisvc.NewAIProviderProfileService()
	if err != nil {
		return nil
	}
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"provider":            aimodels.AIProviderTypeOpenAI,
		"status":              aimodels.AIProviderProfileStatusActive,
	}
	p, err := profileSvc.FindOne(ctx, filter, nil)
	if err != nil {
		systemOrgID, errSys := cta.GetSystemOrganizationID(ctx)
		if errSys != nil {
			return nil
		}
		filter = bson.M{
			"ownerOrganizationId": systemOrgID,
			"name":                aidecisionOpenAIProfileName,
			"provider":            aimodels.AIProviderTypeOpenAI,
		}
		p, err = profileSvc.FindOne(ctx, filter, nil)
		if err != nil {
			return nil
		}
	}
	if strings.TrimSpace(p.APIKey) == "" {
		return nil
	}
	model := defaultAIDecisionLLMModel
	if env := strings.TrimSpace(os.Getenv(envAIDecisionLLMModel)); env != "" {
		model = env
	} else if p.Config != nil && strings.TrimSpace(p.Config.Model) != "" {
		model = p.Config.Model
	} else if len(p.AvailableModels) > 0 {
		model = p.AvailableModels[0]
	}
	return newAIDecisionLLMWithCreds(p.APIKey, model, p.BaseURL)
}

func newAIDecisionLLMWithCreds(apiKey, model, baseURL string) *aidecisionLLMClient {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil
	}
	if model == "" {
		model = defaultAIDecisionLLMModel
	}
	var client *openai.Client
	if strings.TrimSpace(baseURL) != "" {
		config := openai.DefaultConfig(apiKey)
		config.BaseURL = strings.TrimSuffix(baseURL, "/") + "/v1"
		client = openai.NewClientWithConfig(config)
	} else {
		client = openai.NewClient(apiKey)
	}
	return &aidecisionLLMClient{client: client, model: model}
}

func (c *aidecisionLLMClient) available() bool {
	return c != nil && c.client != nil
}

func aidecisionLLMEnabled() bool {
	return strings.TrimSpace(os.Getenv(envAIDecisionLLMEnabled)) == "1"
}

func aidecisionLLMRefineEnabled() bool {
	return strings.TrimSpace(os.Getenv(envAIDecisionLLMRefine)) == "1"
}

func defaultAllowedActionsForLLM() []string {
	raw := strings.TrimSpace(os.Getenv(envAIDecisionLLMAllowedActions))
	if raw == "" {
		raw = "escalate_to_senior,assign_to_human_sale,prioritize_followup,send_promotion,apply_discount"
	}
	var out []string
	for _, a := range strings.Split(raw, ",") {
		a = strings.TrimSpace(strings.ToLower(a))
		if a != "" {
			out = append(out, a)
		}
	}
	return out
}

// resolveActionsWithLLM khi rule/CIX không đủ hoặc cần tinh chỉnh danh sách.
func resolveActionsWithLLM(ctx context.Context, ownerOrgID primitive.ObjectID, req *ExecuteRequest, ruleActions []string, caseDoc *aidecisionmodels.DecisionCase) ([]string, *llmDecisionResult) {
	if !aidecisionLLMEnabled() {
		return ruleActions, nil
	}
	llm := newAIDecisionLLMFromProfile(ctx, ownerOrgID)
	if llm == nil {
		llm = newAIDecisionLLMFromEnv()
	}
	if !llm.available() {
		logrus.Debug("AI Decision LLM: không có client (thiếu profile/env OPENAI_API_KEY)")
		return ruleActions, nil
	}
	allowed := defaultAllowedActionsForLLM()

	// 1) Không có gợi ý từ CIX — LLM chọn từ allowed theo context.
	if len(ruleActions) == 0 {
		if req.TraceID != "" {
			ev := livecopy.BuildLLMEmptySuggestions(req.CorrelationID)
			cid, ctid := "", ""
			if caseDoc != nil {
				cid, ctid = caseDoc.DecisionCaseID, caseDoc.TraceID
			}
			decisionlive.EnrichLiveEventFromCase(cid, ctid, &ev)
			decisionlive.Publish(ownerOrgID, req.TraceID, ev)
		}
		res, err := llm.decideWhenEmpty(ctx, req, allowed)
		if err != nil {
			logrus.WithError(err).Warn("AI Decision LLM: decideWhenEmpty thất bại, giữ danh sách rỗng")
			return nil, nil
		}
		return res.Actions, res
	}

	// 2) Nhiều gợi ý — tinh chỉnh (subset) khi bật refine.
	if len(ruleActions) >= 2 && aidecisionLLMRefineEnabled() {
		if req.TraceID != "" {
			ev := livecopy.BuildLLMRefineSuggestions(req.CorrelationID, len(ruleActions))
			cid, ctid := "", ""
			if caseDoc != nil {
				cid, ctid = caseDoc.DecisionCaseID, caseDoc.TraceID
			}
			decisionlive.EnrichLiveEventFromCase(cid, ctid, &ev)
			decisionlive.Publish(ownerOrgID, req.TraceID, ev)
		}
		res, err := llm.refineActions(ctx, req, ruleActions, allowed)
		if err != nil {
			logrus.WithError(err).Warn("AI Decision LLM: refine thất bại, giữ gợi ý rule")
			return ruleActions, nil
		}
		return res.Actions, res
	}

	return ruleActions, nil
}

func (c *aidecisionLLMClient) decideWhenEmpty(ctx context.Context, req *ExecuteRequest, allowed []string) (*llmDecisionResult, error) {
	ctxJSON, _ := json.Marshal(map[string]interface{}{
		"cixPayload":   req.CIXPayload,
		"customerCtx":  req.CustomerCtx,
		"sessionUid":   req.SessionUid,
		"customerUid":  req.CustomerUid,
		"allowedOnly":  allowed,
	})
	sys := `Bạn là tầng AI Decision (Commerce). Chỉ chọn action từ danh sách allowedOnly.
Trả về ĐÚNG một JSON (không markdown), schema:
{"selected_actions":["action_id",...],"confidence":0.0-1.0,"reasoning_summary":"tiếng Việt ngắn"}
selected_actions: 0-3 phần tử, chỉ giá trị nằm trong allowedOnly.`

	user := fmt.Sprintf("Ngữ cảnh (JSON):\n%s", string(ctxJSON))
	return c.chatJSON(ctx, sys, user, "llm", allowed, nil)
}

func (c *aidecisionLLMClient) refineActions(ctx context.Context, req *ExecuteRequest, current, allowed []string) (*llmDecisionResult, error) {
	allowedUnion := append(append([]string{}, allowed...), current...)
	ctxJSON, _ := json.Marshal(map[string]interface{}{
		"cixPayload":         req.CIXPayload,
		"customerCtx":        req.CustomerCtx,
		"sessionUid":         req.SessionUid,
		"currentSuggestions": current,
		"allowedActions":     allowedUnion,
	})
	sys := `Bạn là tầng AI Decision. Có danh sách gợi ý currentSuggestions từ CIX/Rule.
Chọn một TẬP CON của currentSuggestions (chỉ được chọn action có trong currentSuggestions).
Trả về ĐÚNG JSON:
{"selected_actions":["..."],"confidence":0.0-1.0,"reasoning_summary":"tiếng Việt ngắn"}`

	user := fmt.Sprintf("Ngữ cảnh (JSON):\n%s", string(ctxJSON))
	return c.chatJSON(ctx, sys, user, "hybrid", allowedUnion, current)
}

func (c *aidecisionLLMClient) chatJSON(ctx context.Context, systemPrompt, userContent, mode string, allowedCatalog []string, mustBeSubsetOf []string) (*llmDecisionResult, error) {
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userContent},
		},
		Temperature: 0.25,
		MaxTokens:   512,
	}
	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("LLM không trả nội dung")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var raw llmDecisionJSON
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("JSON LLM không hợp lệ: %w", err)
	}
	actions := normalizeActionList(raw.SelectedActions)
	actions = filterLLMActionsByPolicy(actions, allowedCatalog, mustBeSubsetOf)
	if len(actions) == 0 {
		return &llmDecisionResult{Actions: nil, Confidence: raw.Confidence, ReasoningSummary: raw.ReasoningSummary, Mode: mode}, nil
	}
	return &llmDecisionResult{
		Actions:          actions,
		Confidence:       raw.Confidence,
		ReasoningSummary: strings.TrimSpace(raw.ReasoningSummary),
		Mode:             mode,
	}, nil
}

// filterLLMActionsByPolicy: mọi action phải nằm trong allowedCatalog; nếu mustBeSubsetOf khác rỗng thì còn phải thuộc tập đó (refine).
func filterLLMActionsByPolicy(actions []string, allowedCatalog []string, mustBeSubsetOf []string) []string {
	allow := make(map[string]struct{})
	for _, a := range allowedCatalog {
		allow[normActionKey(a)] = struct{}{}
	}
	var must map[string]struct{}
	if len(mustBeSubsetOf) > 0 {
		must = make(map[string]struct{})
		for _, a := range mustBeSubsetOf {
			must[normActionKey(a)] = struct{}{}
		}
	}
	var out []string
	seen := map[string]struct{}{}
	for _, a := range actions {
		k := normActionKey(a)
		if _, ok := allow[k]; !ok {
			continue
		}
		if must != nil {
			if _, ok := must[k]; !ok {
				continue
			}
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func normActionKey(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

func normalizeActionList(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, a := range in {
		a = strings.TrimSpace(strings.ToLower(a))
		if a == "" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	return out
}
