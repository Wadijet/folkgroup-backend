package dto

// AIProviderProfileCreateInput dữ liệu đầu vào khi tạo AI provider profile
type AIProviderProfileCreateInput struct {
	Name        string `json:"name" validate:"required"`                                                                               // Tên profile (ví dụ: "OpenAI Production", "Claude Dev")
	Description string `json:"description,omitempty"`                                                                                  // Mô tả profile
	Provider    string `json:"provider" validate:"required,oneof=openai anthropic google cohere custom"`                               // Provider type: "openai", "anthropic", "google", etc.
	Status      string `json:"status,omitempty" transform:"string,default=active" validate:"omitempty,oneof=active inactive archived"` // Trạng thái: "active", "inactive", "archived" (mặc định: "active")

	// ===== AUTHENTICATION =====
	APIKey         string `json:"apiKey" validate:"required"` // API key
	BaseURL        string `json:"baseUrl,omitempty"`          // Base URL của API (nếu custom)
	OrganizationID string `json:"organizationId,omitempty"`   // Organization ID (cho OpenAI organization billing)

	// ===== CONFIGURATION =====
	AvailableModels []string       `json:"availableModels,omitempty"`                  // Danh sách models có sẵn
	Config          *AIConfigInput `json:"config,omitempty" transform:"nested_struct"` // AI config (model, temperature, maxTokens, providerConfig, pricingConfig) - dùng chung cho default config

	// ===== RATE LIMITS =====
	RateLimitRequestsPerMinute *int `json:"rateLimitRequestsPerMinute,omitempty"` // Rate limit: requests per minute
	RateLimitTokensPerMinute   *int `json:"rateLimitTokensPerMinute,omitempty"`   // Rate limit: tokens per minute

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty"` // Metadata bổ sung
}

// AIProviderProfileUpdateInput dữ liệu đầu vào khi cập nhật AI provider profile
type AIProviderProfileUpdateInput struct {
	Name        string `json:"name,omitempty"`        // Tên profile
	Description string `json:"description,omitempty"` // Mô tả profile
	Status      string `json:"status,omitempty"`      // Trạng thái: "active", "inactive", "archived"

	// ===== AUTHENTICATION =====
	APIKey         string `json:"apiKey,omitempty"`         // API key (nếu cần update)
	BaseURL        string `json:"baseUrl,omitempty"`        // Base URL
	OrganizationID string `json:"organizationId,omitempty"` // Organization ID

	// ===== CONFIGURATION =====
	AvailableModels []string       `json:"availableModels,omitempty"`                  // Danh sách models có sẵn
	Config          *AIConfigInput `json:"config,omitempty" transform:"nested_struct"` // AI config (model, temperature, maxTokens, providerConfig, pricingConfig) - dùng chung cho default config

	// ===== RATE LIMITS =====
	RateLimitRequestsPerMinute *int `json:"rateLimitRequestsPerMinute,omitempty"` // Rate limit: requests per minute
	RateLimitTokensPerMinute   *int `json:"rateLimitTokensPerMinute,omitempty"`   // Rate limit: tokens per minute

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty"` // Metadata bổ sung
}
