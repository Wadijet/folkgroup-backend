package aidto

// AIProviderProfileCreateInput dữ liệu đầu vào khi tạo AI provider profile
type AIProviderProfileCreateInput struct {
	Name        string         `json:"name" validate:"required"`
	Description string         `json:"description,omitempty"`
	Provider    string         `json:"provider" validate:"required,oneof=openai anthropic google cohere custom"`
	Status      string         `json:"status,omitempty" transform:"string,default=active" validate:"omitempty,oneof=active inactive archived"`
	APIKey      string         `json:"apiKey" validate:"required"`
	BaseURL     string         `json:"baseUrl,omitempty"`
	OrganizationID string      `json:"organizationId,omitempty"`
	AvailableModels []string   `json:"availableModels,omitempty"`
	Config      *AIConfigInput `json:"config,omitempty" transform:"nested_struct"`
	RateLimitRequestsPerMinute *int `json:"rateLimitRequestsPerMinute,omitempty"`
	RateLimitTokensPerMinute   *int `json:"rateLimitTokensPerMinute,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AIProviderProfileUpdateInput dữ liệu đầu vào khi cập nhật AI provider profile
type AIProviderProfileUpdateInput struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Status      string         `json:"status,omitempty"`
	APIKey      string         `json:"apiKey,omitempty"`
	BaseURL     string         `json:"baseUrl,omitempty"`
	OrganizationID string      `json:"organizationId,omitempty"`
	AvailableModels []string   `json:"availableModels,omitempty"`
	Config      *AIConfigInput `json:"config,omitempty" transform:"nested_struct"`
	RateLimitRequestsPerMinute *int `json:"rateLimitRequestsPerMinute,omitempty"`
	RateLimitTokensPerMinute   *int `json:"rateLimitTokensPerMinute,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
