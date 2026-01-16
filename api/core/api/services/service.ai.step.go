package services

import (
	"context"
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIStepService là service quản lý AI steps (Module 2)
type AIStepService struct {
	*BaseServiceMongoImpl[models.AIStep]
}

// NewAIStepService tạo mới AIStepService
// Trả về:
//   - *AIStepService: Instance mới của AIStepService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewAIStepService() (*AIStepService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AISteps)
	if !exist {
		return nil, fmt.Errorf("failed to get ai_steps collection: %v", common.ErrNotFound)
	}

	return &AIStepService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.AIStep](collection),
	}, nil
}

// RenderPromptForStep render prompt template cho step với variables và resolve AI config
// Tham số:
//   - ctx: Context
//   - stepID: ID của step
//   - variables: Map các biến và giá trị để thay thế vào prompt (từ step input)
//
// Trả về:
//   - renderedPrompt: Prompt đã được render (TEXT)
//   - providerProfileID: ID của provider profile (đã resolve)
//   - provider: Tên provider
//   - model: Model name (đã resolve)
//   - temperature: Temperature (đã resolve)
//   - maxTokens: Max tokens (đã resolve)
//   - error: Lỗi nếu có
func (s *AIStepService) RenderPromptForStep(ctx context.Context, stepID primitive.ObjectID, variables map[string]interface{}) (
	renderedPrompt string,
	providerProfileID string,
	provider string,
	model string,
	temperature *float64,
	maxTokens *int,
	err error,
) {
	// 1. Load step
	step, err := s.FindOneById(ctx, stepID)
	if err != nil {
		return "", "", "", "", nil, nil, fmt.Errorf("failed to load step: %w", err)
	}

	// 2. Kiểm tra step có prompt template không
	if step.PromptTemplateID == nil {
		return "", "", "", "", nil, nil, fmt.Errorf("step does not have prompt template")
	}

	// 3. Load prompt template
	promptTemplateService, err := NewAIPromptTemplateService()
	if err != nil {
		return "", "", "", "", nil, nil, fmt.Errorf("failed to create prompt template service: %w", err)
	}

	template, err := promptTemplateService.FindOneById(ctx, *step.PromptTemplateID)
	if err != nil {
		return "", "", "", "", nil, nil, fmt.Errorf("failed to load prompt template: %w", err)
	}

	// 4. Render prompt (cần pointer)
	renderedPrompt, err = promptTemplateService.RenderPrompt(&template, variables)
	if err != nil {
		return "", "", "", "", nil, nil, fmt.Errorf("failed to render prompt: %w", err)
	}

	// 5. Resolve AI config (logic 2 lớp: prompt template override provider default)
	var finalProviderProfileID *primitive.ObjectID
	var finalModel string
	var finalTemperature *float64
	var finalMaxTokens *int
	var finalProvider string

	// 5a. Resolve provider profile từ Provider
	if template.Provider != nil && template.Provider.ProfileID != nil {
		// Prompt template có providerProfileId → dùng provider đó
		finalProviderProfileID = template.Provider.ProfileID
	} else {
		// Prompt template không có → cần tìm default provider (có thể skip nếu không có)
		// LƯU Ý: Hiện tại không có logic để tìm default provider của organization
		// Nếu cần trong tương lai, có thể implement bằng cách:
		// 1. Query AIProviderProfile với ownerOrganizationId và status = "active"
		// 2. Chọn provider đầu tiên hoặc provider có isDefault = true
		// 3. Fallback về System Organization provider nếu không có
	}

	// 5b. Load provider profile để lấy default config
	if finalProviderProfileID != nil {
		providerProfileService, err := NewAIProviderProfileService()
		if err == nil {
			providerProfile, err := providerProfileService.FindOneById(ctx, *finalProviderProfileID)
			if err == nil {
				finalProvider = providerProfile.Provider

				// 5c. Resolve model: prompt template override provider default
				if template.Provider != nil && template.Provider.Config != nil && template.Provider.Config.Model != "" {
					finalModel = template.Provider.Config.Model
				} else if providerProfile.Config != nil && providerProfile.Config.Model != "" {
					finalModel = providerProfile.Config.Model
				}

				// 5d. Resolve temperature: prompt template override provider default
				if template.Provider != nil && template.Provider.Config != nil && template.Provider.Config.Temperature != nil {
					finalTemperature = template.Provider.Config.Temperature
				} else if providerProfile.Config != nil {
					finalTemperature = providerProfile.Config.Temperature
				}

				// 5e. Resolve maxTokens: prompt template override provider default
				if template.Provider != nil && template.Provider.Config != nil && template.Provider.Config.MaxTokens != nil {
					finalMaxTokens = template.Provider.Config.MaxTokens
				} else if providerProfile.Config != nil {
					finalMaxTokens = providerProfile.Config.MaxTokens
				}
			}
		}
	} else {
		// Không có provider profile → chỉ dùng config từ prompt template
		if template.Provider != nil && template.Provider.Config != nil {
			finalModel = template.Provider.Config.Model
			finalTemperature = template.Provider.Config.Temperature
			finalMaxTokens = template.Provider.Config.MaxTokens
		}
	}

	// 6. Convert providerProfileID sang string
	providerProfileIDStr := ""
	if finalProviderProfileID != nil {
		providerProfileIDStr = finalProviderProfileID.Hex()
	}

	return renderedPrompt, providerProfileIDStr, finalProvider, finalModel, finalTemperature, finalMaxTokens, nil
}

// ValidateSchema validate input/output schema với standard schema (business logic validation)
//
// LÝ DO PHẢI TẠO METHOD NÀY (không dùng CRUD base):
// 1. Business rules - Schema validation:
//    - Validate input/output schema phải match với standard schema cho từng step type
//    - Đảm bảo mapping chính xác giữa output của step này và input của step tiếp theo
//    - Cho phép mở rộng thêm fields nhưng không được thiếu required fields
//    - Đây là business logic validation phức tạp, không thể dùng struct tag
//
// Tham số:
//   - stepType: Loại step (ví dụ: "generate_content", "analyze_sentiment")
//   - inputSchema: Input schema của step
//   - outputSchema: Output schema của step
//
// Trả về:
//   - error: Lỗi nếu validation thất bại, nil nếu hợp lệ
func (s *AIStepService) ValidateSchema(stepType string, inputSchema map[string]interface{}, outputSchema map[string]interface{}) error {
	isValid, errors := models.ValidateStepSchema(stepType, inputSchema, outputSchema)
	if !isValid {
		return common.NewError(
			common.ErrCodeValidationFormat,
			fmt.Sprintf("Schema không hợp lệ. Chi tiết: %v", errors),
			common.StatusBadRequest,
			nil,
		)
	}
	return nil
}

// InsertOne override để thêm business logic validation trước khi insert
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseServiceMongoImpl.InsertOne trực tiếp):
// 1. Business logic validation:
//    - Validate input/output schema với standard schema
//    - Đảm bảo schema hợp lệ trước khi insert
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Validate schema bằng ValidateSchema()
// ✅ Gọi BaseServiceMongoImpl.InsertOne để đảm bảo:
//   - Set timestamps (CreatedAt, UpdatedAt)
//   - Generate ID nếu chưa có
//   - Insert vào MongoDB
func (s *AIStepService) InsertOne(ctx context.Context, data models.AIStep) (models.AIStep, error) {
	// Validate schema (business logic validation)
	// Convert InputSchema và OutputSchema từ bson.M sang map[string]interface{}
	var inputSchema map[string]interface{}
	var outputSchema map[string]interface{}
	
	if data.InputSchema != nil {
		inputSchema = data.InputSchema
	}
	if data.OutputSchema != nil {
		outputSchema = data.OutputSchema
	}
	
	if err := s.ValidateSchema(data.Type, inputSchema, outputSchema); err != nil {
		return data, err
	}

	// Gọi InsertOne của base service
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}
