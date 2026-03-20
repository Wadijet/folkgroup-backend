// Package service — LLM Layer 3 cho CIX.
//
// ExtractLayer3Signals dùng OpenAI (hoặc tương thích) để trích xuất buyingIntent,
// sentiment, objectionLevel từ nội dung hội thoại. Thay thế/bổ sung Rule Engine cho Layer 3.
//
// API key và model: ưu tiên từ ai_provider_profiles (DB), fallback env OPENAI_API_KEY, CIX_LLM_MODEL.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	aimodels "meta_commerce/internal/api/ai/models"
	aisvc "meta_commerce/internal/api/ai/service"
	cixmodels "meta_commerce/internal/api/cix/models"
	"meta_commerce/internal/cta"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	openai "github.com/sashabaranov/go-openai"
)

const (
	// EnvOpenAIAPIKey biến môi trường cho OpenAI API key (fallback khi không có profile trong DB).
	EnvOpenAIAPIKey = "OPENAI_API_KEY"
	// EnvCIXLLMModel model mặc định cho CIX Layer 3 (vd: gpt-4o-mini, gpt-4).
	EnvCIXLLMModel = "CIX_LLM_MODEL"
	// DefaultCIXLLMModel model mặc định — rẻ, đủ tốt cho extraction.
	DefaultCIXLLMModel = "gpt-4o-mini"
	// CIXDefaultProviderProfileName tên profile OpenAI mặc định của system org.
	CIXDefaultProviderProfileName = "OpenAI Production"
)

// CixLLMService service gọi LLM để trích xuất Layer 3 signals.
type CixLLMService struct {
	client *openai.Client
	model  string
}

// NewCixLLMService tạo service từ env (OPENAI_API_KEY, CIX_LLM_MODEL). Trả nil nếu không có API key.
func NewCixLLMService() *CixLLMService {
	apiKey := strings.TrimSpace(os.Getenv(EnvOpenAIAPIKey))
	if apiKey == "" {
		return nil
	}
	model := strings.TrimSpace(os.Getenv(EnvCIXLLMModel))
	if model == "" {
		model = DefaultCIXLLMModel
	}
	return newCixLLMServiceWithCreds(apiKey, model, "")
}

// NewCixLLMServiceFromProfile tạo service từ ai_provider_profiles trong DB.
// Ưu tiên: profile của org (ownerOrgID, provider=openai, status=active) → system org "OpenAI Production".
// Trả nil nếu không tìm thấy profile có API key.
func NewCixLLMServiceFromProfile(ctx context.Context, ownerOrgID primitive.ObjectID) *CixLLMService {
	profileSvc, err := aisvc.NewAIProviderProfileService()
	if err != nil {
		return nil
	}

	// 1. Thử profile của org: ownerOrgID + provider=openai + status=active
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"provider":            aimodels.AIProviderTypeOpenAI,
		"status":              aimodels.AIProviderProfileStatusActive,
	}
	p, err := profileSvc.FindOne(ctx, filter, nil)
	if err != nil {
		// 2. Fallback: system org "OpenAI Production"
		systemOrgID, errSys := cta.GetSystemOrganizationID(ctx)
		if errSys != nil {
			return nil
		}
		filter = bson.M{
			"ownerOrganizationId": systemOrgID,
			"name":                CIXDefaultProviderProfileName,
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

	model := DefaultCIXLLMModel
	if p.Config != nil && strings.TrimSpace(p.Config.Model) != "" {
		model = p.Config.Model
	} else if len(p.AvailableModels) > 0 {
		model = p.AvailableModels[0]
	}

	return newCixLLMServiceWithCreds(p.APIKey, model, p.BaseURL)
}

// newCixLLMServiceWithCreds tạo service từ apiKey, model, baseURL (baseURL rỗng = dùng mặc định OpenAI).
func newCixLLMServiceWithCreds(apiKey, model, baseURL string) *CixLLMService {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil
	}
	if model == "" {
		model = DefaultCIXLLMModel
	}

	var client *openai.Client
	if strings.TrimSpace(baseURL) != "" {
		config := openai.DefaultConfig(apiKey)
		config.BaseURL = strings.TrimSuffix(baseURL, "/") + "/v1"
		client = openai.NewClientWithConfig(config)
	} else {
		client = openai.NewClient(apiKey)
	}

	return &CixLLMService{
		client: client,
		model:  model,
	}
}

// IsAvailable trả true nếu service có thể gọi LLM (đã cấu hình API key).
func (s *CixLLMService) IsAvailable() bool {
	return s != nil && s.client != nil
}

// layer3LLMOutput cấu trúc JSON trả về từ LLM.
type layer3LLMOutput struct {
	BuyingIntent   string `json:"buyingIntent"`
	ObjectionLevel string `json:"objectionLevel"`
	Sentiment      string `json:"sentiment"`
}

// ExtractLayer3Signals trích xuất Layer 3 (buyingIntent, sentiment, objectionLevel) từ hội thoại bằng LLM.
// turns: danh sách lượt chat [{from, content, timestamp}].
// customerCtx: valueTier, lifecycleStage, journeyStage (context khách).
func (s *CixLLMService) ExtractLayer3Signals(ctx context.Context, turns []map[string]interface{}, customerCtx map[string]interface{}) (*cixmodels.CixLayer3, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("CIX LLM chưa cấu hình: thiếu %s", EnvOpenAIAPIKey)
	}
	if len(turns) == 0 {
		return &cixmodels.CixLayer3{
			BuyingIntent:   "none",
			ObjectionLevel: "none",
			Sentiment:      "neutral",
		}, nil
	}

	// Build transcript text
	var sb strings.Builder
	for _, t := range turns {
		from, _ := t["from"].(string)
		content, _ := t["content"].(string)
		if from == "" {
			from = "unknown"
		}
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", from, content))
	}
	transcript := sb.String()
	if transcript == "" {
		return &cixmodels.CixLayer3{
			BuyingIntent:   "none",
			ObjectionLevel: "none",
			Sentiment:      "neutral",
		}, nil
	}

	// Customer context string
	ctxStr := ""
	if customerCtx != nil {
		if v, ok := customerCtx["valueTier"].(string); ok && v != "" {
			ctxStr += fmt.Sprintf("valueTier: %s; ", v)
		}
		if v, ok := customerCtx["lifecycleStage"].(string); ok && v != "" {
			ctxStr += fmt.Sprintf("lifecycleStage: %s; ", v)
		}
		if v, ok := customerCtx["journeyStage"].(string); ok && v != "" {
			ctxStr += fmt.Sprintf("journeyStage: %s", v)
		}
	}
	if ctxStr != "" {
		ctxStr = "\n\nNgữ cảnh khách hàng: " + ctxStr
	}

	prompt := `Phân tích cuộc hội thoại sau và trích xuất 3 tín hiệu. Trả về ĐÚNG định dạng JSON:
{"buyingIntent":"...","objectionLevel":"...","sentiment":"..."}

Quy tắc:
- buyingIntent: none | inquiring | ready_to_buy
- objectionLevel: none | soft_objection | hard_objection
- sentiment: positive | neutral | negative | angry

Chỉ trả về JSON, không thêm text khác.`

	userContent := fmt.Sprintf("%s\n\n=== HỘI THOẠI ===\n%s%s", prompt, transcript, ctxStr)

	req := openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "Bạn là chuyên gia phân tích hội thoại bán hàng. Trả về JSON chính xác theo yêu cầu."},
			{Role: openai.ChatMessageRoleUser, Content: userContent},
		},
		Temperature: 0.2,
		MaxTokens:   256,
	}

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLM gọi thất bại: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("LLM không trả về nội dung")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	// Loại bỏ markdown code block nếu có
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var out layer3LLMOutput
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("LLM trả JSON không hợp lệ: %w", err)
	}

	// Chuẩn hóa giá trị về enum hợp lệ
	return &cixmodels.CixLayer3{
		BuyingIntent:   normalizeBuyingIntent(out.BuyingIntent),
		ObjectionLevel: normalizeObjectionLevel(out.ObjectionLevel),
		Sentiment:      normalizeSentiment(out.Sentiment),
	}, nil
}

func normalizeBuyingIntent(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "none", "inquiring", "ready_to_buy", "ready to buy":
		if s == "ready to buy" {
			return "ready_to_buy"
		}
		return s
	default:
		return "none"
	}
}

func normalizeObjectionLevel(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "none", "soft_objection", "soft objection", "hard_objection", "hard objection":
		if s == "soft objection" {
			return "soft_objection"
		}
		if s == "hard objection" {
			return "hard_objection"
		}
		return s
	default:
		return "none"
	}
}

func normalizeSentiment(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "positive", "neutral", "negative", "angry":
		return s
	default:
		return "neutral"
	}
}
