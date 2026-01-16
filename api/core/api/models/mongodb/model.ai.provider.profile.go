package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIProviderType định nghĩa các loại AI provider
const (
	AIProviderTypeOpenAI    = "openai"    // OpenAI (GPT-4, GPT-3.5, etc.)
	AIProviderTypeAnthropic = "anthropic" // Anthropic (Claude)
	AIProviderTypeGoogle    = "google"    // Google (Gemini)
	AIProviderTypeCohere    = "cohere"    // Cohere
	AIProviderTypeCustom    = "custom"    // Custom provider
)

// AIProviderProfileStatus định nghĩa các trạng thái provider profile
const (
	AIProviderProfileStatusActive   = "active"   // Đang hoạt động
	AIProviderProfileStatusInactive = "inactive" // Không hoạt động
	AIProviderProfileStatusArchived = "archived" // Đã lưu trữ
)

// AIConfig chứa AI config chung (dùng cho cả default config và override config)
type AIConfig struct {
	Model          string                 `json:"model,omitempty" bson:"model,omitempty"`                   // Model name (ví dụ: "gpt-4", "claude-3-opus")
	Temperature    *float64               `json:"temperature,omitempty" bson:"temperature,omitempty"`       // Temperature
	MaxTokens      *int                   `json:"maxTokens,omitempty" bson:"maxTokens,omitempty"`           // Max tokens
	ProviderConfig map[string]interface{} `json:"providerConfig,omitempty" bson:"providerConfig,omitempty"` // Provider-specific config (ví dụ: OpenAI topP, Anthropic maxTokensToSample)
	// Ví dụ ProviderConfig:
	// - OpenAI: {"topP": 1.0, "frequencyPenalty": 0.0, "presencePenalty": 0.0}
	// - Anthropic: {"maxTokensToSample": 4096, "stopSequences": []}
	// - Google: {"topK": 40, "topP": 0.95}
	PricingConfig map[string]interface{} `json:"pricingConfig,omitempty" bson:"pricingConfig,omitempty"` // Pricing config: {"gpt-4": {"input": 0.03, "output": 0.06}, ...}
}

// AIProviderProfile đại diện cho AI provider profile (Module 2)
// Collection: ai_provider_profiles
// Lưu thông tin về AI provider: API keys, config, models, pricing
type AIProviderProfile struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của provider profile

	// ===== BASIC INFO =====
	Name        string `json:"name" bson:"name" index:"text"`                      // Tên profile (ví dụ: "OpenAI Production", "Claude Dev")
	Description string `json:"description,omitempty" bson:"description,omitempty"` // Mô tả profile
	Provider    string `json:"provider" bson:"provider" index:"single:1"`          // Provider type: "openai", "anthropic", "google", etc.
	Status      string `json:"status" bson:"status" index:"single:1"`              // Trạng thái: "active", "inactive", "archived"

	// ===== AUTHENTICATION =====
	APIKey          string `json:"apiKey" bson:"apiKey"`                                       // API key (nên được encrypt khi lưu)
	APIKeyEncrypted bool   `json:"apiKeyEncrypted,omitempty" bson:"apiKeyEncrypted,omitempty"` // Flag để biết API key đã được encrypt chưa
	BaseURL         string `json:"baseUrl,omitempty" bson:"baseUrl,omitempty"`                 // Base URL của API (nếu custom)
	OrganizationID  string `json:"organizationId,omitempty" bson:"organizationId,omitempty"`   // Organization ID (cho OpenAI organization billing)

	// ===== CONFIGURATION =====
	AvailableModels []string  `json:"availableModels,omitempty" bson:"availableModels,omitempty"` // Danh sách models có sẵn
	Config          *AIConfig `json:"config,omitempty" bson:"config,omitempty"`                   // AI config (model, temperature, maxTokens, providerConfig, pricingConfig) - dùng chung cho default config

	// ===== RATE LIMITS =====
	RateLimitRequestsPerMinute *int `json:"rateLimitRequestsPerMinute,omitempty" bson:"rateLimitRequestsPerMinute,omitempty"` // Rate limit: requests per minute
	RateLimitTokensPerMinute   *int `json:"rateLimitTokensPerMinute,omitempty" bson:"rateLimitTokensPerMinute,omitempty"`     // Rate limit: tokens per minute

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu provider profile

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"` // Metadata bổ sung

	// ===== TIMESTAMPS =====
	CreatedAt int64 `json:"createdAt" bson:"createdAt" index:"single:1"`    // Thời gian tạo
	UpdatedAt int64 `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"` // Thời gian cập nhật
}
