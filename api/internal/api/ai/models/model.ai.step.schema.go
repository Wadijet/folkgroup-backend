package models

import "fmt"

// AIStepStandardSchemas định nghĩa input/output schema chuẩn cho từng loại step
// Mục đích: Đảm bảo mapping chính xác giữa output của step này và input của step tiếp theo
//
// QUAN TRỌNG VỀ AI INPUT/OUTPUT:
// - AI Input (prompt): CHỈ là TEXT - được generate từ step input data
// - AI Output (response): CHỈ là TEXT - raw response từ AI API
// - System sẽ tự động:
//   + Parse AI response text → structured data theo outputSchema
//   + Bổ sung metadata: timestamps, tokens, model, cost, etc.
//   + Tạo content, score, feedback, etc. từ parsed output
//
// QUAN TRỌNG VỀ STEP INPUT/OUTPUT:
// - Step Input: Dữ liệu đầu vào cho step (pillarId, context, etc.) - dùng để generate prompt
// - Step Output: Dữ liệu đầu ra của step (content, score, etc.) - bao gồm parsed AI output + system metadata
// - Khi execute workflow, output của step này sẽ được map vào input của step tiếp theo

// GetStandardInputSchema trả về input schema chuẩn cho từng loại step
func GetStandardInputSchema(stepType string) map[string]interface{} {
	switch stepType {
	case AIStepTypeGenerate:
		return GetStandardGenerateInputSchema()
	case AIStepTypeJudge:
		return GetStandardJudgeInputSchema()
	case AIStepTypeStepGeneration:
		return GetStandardStepGenerationInputSchema()
	default:
		return nil
	}
}

// GetStandardOutputSchema trả về output schema chuẩn cho từng loại step
// DEPRECATED: Dùng GetStandardSchema() với targetLevel và parentLevel để đảm bảo consistency
func GetStandardOutputSchema(stepType string) map[string]interface{} {
	switch stepType {
	case AIStepTypeGenerate:
		return GetStandardGenerateOutputSchema()
	case AIStepTypeJudge:
		return GetStandardJudgeOutputSchema()
	case AIStepTypeStepGeneration:
		return GetStandardStepGenerationOutputSchema()
	default:
		return nil
	}
}

// GetStandardSchema trả về input và output schema chuẩn theo (stepType + TargetLevel + ParentLevel)
// Đảm bảo schema nhất quán giữa các steps cùng level, tránh phá vỡ rule giữa các level
//
// Parameters:
//   - stepType: Loại step (GENERATE, JUDGE, STEP_GENERATION)
//   - targetLevel: Level mục tiêu (L1, L2, ..., L8) - có thể rỗng
//   - parentLevel: Level của parent (L1, L2, ..., L8) - có thể rỗng
//
// Returns:
//   - inputSchema: Input schema chuẩn
//   - outputSchema: Output schema chuẩn
//   - error: Lỗi nếu có
//
// Logic:
//   - GENERATE step: Schema phụ thuộc vào TargetLevel và ParentLevel
//   - L1 (no parent): Schema đặc biệt cho Pillar
//   - L2-L8 (có parent): Schema chung cho tất cả level transitions
//   - JUDGE step: Schema giống nhau cho tất cả level (chỉ đánh giá 1 nội dung)
//   - STEP_GENERATION step: Schema giống nhau cho tất cả level
func GetStandardSchema(stepType, targetLevel, parentLevel string) (map[string]interface{}, map[string]interface{}, error) {
	var inputSchema map[string]interface{}
	var outputSchema map[string]interface{}

	switch stepType {
	case AIStepTypeGenerate:
		// GENERATE step: Schema phụ thuộc vào level
		if targetLevel == "L1" && parentLevel == "" {
			// L1 (Pillar): Không có parent, schema đặc biệt
			inputSchema = GetStandardGenerateInputSchema()
			outputSchema = GetStandardGenerateOutputSchema()
		} else if targetLevel != "" {
			// L2-L8: Có parent, schema chung (có thể customize theo level nếu cần)
			inputSchema = GetStandardGenerateInputSchema()
			outputSchema = GetStandardGenerateOutputSchema()
		} else {
			// Fallback: Dùng schema chung
			inputSchema = GetStandardGenerateInputSchema()
			outputSchema = GetStandardGenerateOutputSchema()
		}

	case AIStepTypeJudge:
		// JUDGE step: Schema giống nhau cho tất cả level (chỉ đánh giá 1 nội dung)
		inputSchema = GetStandardJudgeInputSchema()
		outputSchema = GetStandardJudgeOutputSchema()

	case AIStepTypeStepGeneration:
		// STEP_GENERATION step: Schema giống nhau cho tất cả level
		inputSchema = GetStandardStepGenerationInputSchema()
		outputSchema = GetStandardStepGenerationOutputSchema()

	default:
		return nil, nil, fmt.Errorf("step type không hợp lệ: %s", stepType)
	}

	return inputSchema, outputSchema, nil
}

// GetStandardGenerateInputSchema trả về input schema chuẩn cho GENERATE step
//
// LƯU Ý:
// - Lớp AI: AI chỉ nhận TEXT (prompt) → trả về TEXT (response)
// - Lớp Logic: System tự lấy parent node từ DB, tự build prompt, tự parse response
// - Step Input: parentText (system tự lấy từ parentNode.Text) + metadata (optional)
func GetStandardGenerateInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"parentText"},
		"properties": map[string]interface{}{
			"parentText": map[string]interface{}{
				"type":        "string",
				"description": "Text của parent node (system tự lấy từ parentNode.Text)",
			},
			"metadata": map[string]interface{}{
				"type":        "object",
				"description": "Metadata tùy chọn (targetAudience, tone, etc.)",
			},
		},
	}
}

// GetStandardGenerateOutputSchema trả về output schema chuẩn cho GENERATE step
//
// LƯU Ý:
// - Lớp AI: AI chỉ trả về TEXT (response) - không biết về structure
// - Lớp Logic: System tự parse text → structured data (text, name, summary)
// - System tự bổ sung: generatedAt, model, tokens (lưu trong step run, không cần trong output)
//
// ĐƠN GIẢN HÓA: text (required) + name (optional) + summary (optional)
// - text → node.Text
// - name → node.Name
// - summary → node.Metadata.summary
func GetStandardGenerateOutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"text"},
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Nội dung chính (REQUIRED) - map vào node.Text",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Tên node (optional) - map vào node.Name",
			},
			"summary": map[string]interface{}{
				"type":        "string",
				"description": "Tóm tắt (optional) - lưu vào node.Metadata.summary",
			},
		},
	}
}

// GetStandardJudgeInputSchema trả về input schema chuẩn cho JUDGE step
//
// LƯU Ý:
// - Lớp AI: AI chỉ nhận TEXT (prompt) → trả về TEXT (response)
// - Lớp Logic: System tự lấy text từ GENERATE output, tự build prompt, tự parse response
// - Step Input: text (system lấy từ GENERATE output) + criteria (system tự lấy) + metadata (optional)
func GetStandardJudgeInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"text", "criteria"},
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text cần đánh giá (system lấy từ GENERATE output.text)",
			},
			"criteria": map[string]interface{}{
				"type":        "object",
				"description": "Tiêu chí đánh giá (system tự lấy từ step config)",
				"properties": map[string]interface{}{
					"relevance": map[string]interface{}{
						"type":        "number",
						"description": "Độ liên quan (0-10)",
						"minimum":     0,
						"maximum":     10,
					},
					"clarity": map[string]interface{}{
						"type":        "number",
						"description": "Độ rõ ràng (0-10)",
						"minimum":     0,
						"maximum":     10,
					},
					"engagement": map[string]interface{}{
						"type":        "number",
						"description": "Độ hấp dẫn (0-10)",
						"minimum":     0,
						"maximum":     10,
					},
					"accuracy": map[string]interface{}{
						"type":        "number",
						"description": "Độ chính xác (0-10)",
						"minimum":     0,
						"maximum":     10,
					},
				},
			},
			"metadata": map[string]interface{}{
				"type":        "object",
				"description": "Metadata tùy chọn (name, summary, context, etc.)",
			},
		},
	}
}

// GetStandardJudgeOutputSchema trả về output schema chuẩn cho JUDGE step
//
// LƯU Ý:
// - Lớp AI: AI chỉ trả về TEXT (response) - system tự parse
// - Lớp Logic: System tự parse text → score + metadata. judgedAt lưu trong step run.
func GetStandardJudgeOutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"score"},
		"properties": map[string]interface{}{
			"score": map[string]interface{}{
				"type":        "number",
				"description": "Điểm tổng thể (0-10)",
				"minimum":     0,
				"maximum":     10,
			},
			"metadata": map[string]interface{}{
				"type":        "object",
				"description": "Metadata tùy chọn (feedback, criteriaScores, etc.)",
			},
		},
	}
}

// GetStandardStepGenerationInputSchema trả về input schema chuẩn cho STEP_GENERATION step
//
// LƯU Ý:
// - Step Input: parentContext, requirements, targetLevel, constraints
// - AI Input (prompt): CHỈ là TEXT - được generate từ step input này
// - AI Output (response): CHỈ là TEXT - raw response từ AI
// - System sẽ parse text này → structured data (generatedSteps[], generationPlan)
// - System tự bổ sung: generatedAt, model, tokens (không phải từ AI)
func GetStandardStepGenerationInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"parentContext", "requirements", "targetLevel"},
		"properties": map[string]interface{}{
			"parentContext": map[string]interface{}{
				"type":        "object",
				"description": "Context từ parent pillar/step",
				"properties": map[string]interface{}{
					"pillarId": map[string]interface{}{
						"type":        "string",
						"description": "ID của parent pillar",
					},
					"pillarName": map[string]interface{}{
						"type": "string",
					},
					"pillarType": map[string]interface{}{
						"type": "string",
						"enum": []string{"L1", "L2", "L3", "L4", "L5", "L6", "L7", "L8"},
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Nội dung của parent pillar",
					},
				},
			},
			"requirements": map[string]interface{}{
				"type":        "object",
				"description": "Yêu cầu cho việc generate steps",
				"properties": map[string]interface{}{
					"numberOfSteps": map[string]interface{}{
						"type":        "integer",
						"description": "Số lượng steps cần generate",
						"minimum":     1,
						"maximum":     10,
						"default":     3,
					},
					"stepTypes": map[string]interface{}{
						"type":        "array",
						"description": "Các loại steps muốn generate",
						"items": map[string]interface{}{
							"type": "string",
							"enum": []string{"GENERATE", "JUDGE", "STEP_GENERATION"},
						},
					},
					"focusAreas": map[string]interface{}{
						"type":        "array",
						"description": "Các lĩnh vực tập trung",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"complexity": map[string]interface{}{
						"type":        "string",
						"description": "Độ phức tạp",
						"enum":        []string{"simple", "medium", "complex"},
					},
				},
			},
			"targetLevel": map[string]interface{}{
				"type":        "string",
				"description": "Level mục tiêu cho các steps được generate",
				"enum":        []string{"L1", "L2", "L3", "L4", "L5", "L6", "L7", "L8"},
			},
			"constraints": map[string]interface{}{
				"type":        "object",
				"description": "Ràng buộc cho việc generate",
				"properties": map[string]interface{}{
					"maxExecutionTime": map[string]interface{}{
						"type":        "integer",
						"description": "Thời gian thực thi tối đa (seconds)",
					},
					"requiredOutputs": map[string]interface{}{
						"type":        "array",
						"description": "Các outputs bắt buộc",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"excludedStepTypes": map[string]interface{}{
						"type":        "array",
						"description": "Các loại steps không muốn generate",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
			"metadata": map[string]interface{}{
				"type":        "object",
				"description": "Metadata bổ sung",
				"properties": map[string]interface{}{
					"useCase": map[string]interface{}{
						"type": "string",
					},
					"priority": map[string]interface{}{
						"type": "string",
						"enum": []string{"low", "medium", "high", "critical"},
					},
				},
			},
		},
	}
}

// GetStandardStepGenerationOutputSchema trả về output schema chuẩn cho STEP_GENERATION step
//
// LƯU Ý:
// - AI Output (response): CHỈ là TEXT - raw response từ AI API
// - System sẽ parse text này → structured data (generatedSteps[], generationPlan)
// - System tự bổ sung: generatedAt, model, tokens (không phải từ AI)
func GetStandardStepGenerationOutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"generatedSteps", "generationPlan", "generatedAt"},
		"properties": map[string]interface{}{
			"generatedSteps": map[string]interface{}{
				"type":        "array",
				"description": "Danh sách các steps đã được generate",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"stepId": map[string]interface{}{
							"type":        "string",
							"description": "ID của step đã được tạo",
						},
						"stepName": map[string]interface{}{
							"type": "string",
						},
						"stepType": map[string]interface{}{
							"type": "string",
							"enum": []string{"GENERATE", "JUDGE", "STEP_GENERATION"},
						},
						"order": map[string]interface{}{
							"type":        "integer",
							"description": "Thứ tự trong workflow",
						},
						"inputSchema": map[string]interface{}{
							"type":        "object",
							"description": "Input schema của step",
						},
						"outputSchema": map[string]interface{}{
							"type":        "object",
							"description": "Output schema của step",
						},
						"description": map[string]interface{}{
							"type": "string",
						},
						"dependencies": map[string]interface{}{
							"type":        "array",
							"description": "Các steps phụ thuộc",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
			"generationPlan": map[string]interface{}{
				"type":        "object",
				"description": "Kế hoạch generation",
				"properties": map[string]interface{}{
					"totalSteps": map[string]interface{}{
						"type": "integer",
					},
					"estimatedTime": map[string]interface{}{
						"type":        "integer",
						"description": "Thời gian ước tính (seconds)",
					},
					"workflowStructure": map[string]interface{}{
						"type":        "object",
						"description": "Cấu trúc workflow",
						"properties": map[string]interface{}{
							"parallelSteps": map[string]interface{}{
								"type":        "array",
								"description": "Các steps có thể chạy song song",
							},
							"sequentialSteps": map[string]interface{}{
								"type":        "array",
								"description": "Các steps phải chạy tuần tự",
							},
						},
					},
					"reasoning": map[string]interface{}{
						"type":        "string",
						"description": "Lý do tại sao generate các steps này",
					},
				},
			},
			"generatedAt": map[string]interface{}{
				"type":        "string",
				"format":      "date-time",
				"description": "Thời gian generate",
			},
			"model": map[string]interface{}{
				"type":        "string",
				"description": "Model AI đã sử dụng",
			},
			"tokens": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"input": map[string]interface{}{
						"type": "integer",
					},
					"output": map[string]interface{}{
						"type": "integer",
					},
					"total": map[string]interface{}{
						"type": "integer",
					},
				},
			},
		},
	}
}

// ValidateStepSchema kiểm tra xem schema có match với standard schema không
// Trả về true nếu schema hợp lệ, false nếu không
// Lưu ý: Cho phép mở rộng thêm fields nhưng không được thiếu required fields
func ValidateStepSchema(stepType string, inputSchema, outputSchema map[string]interface{}) (bool, []string) {
	var errors []string

	// Lấy standard schemas
	stdInputSchema := GetStandardInputSchema(stepType)
	stdOutputSchema := GetStandardOutputSchema(stepType)

	if stdInputSchema == nil || stdOutputSchema == nil {
		errors = append(errors, "Step type không hợp lệ hoặc chưa có standard schema")
		return false, errors
	}

	// Validate input schema: Kiểm tra required fields
	if stdRequired, ok := stdInputSchema["required"].([]string); ok {
		if inputRequired, ok := inputSchema["required"].([]string); ok {
			for _, reqField := range stdRequired {
				found := false
				for _, field := range inputRequired {
					if field == reqField {
						found = true
						break
					}
				}
				if !found {
					errors = append(errors, fmt.Sprintf("Input schema thiếu required field: %s", reqField))
				}
			}
		} else {
			errors = append(errors, "Input schema thiếu 'required' fields")
		}
	}

	// Validate output schema: Kiểm tra required fields
	if stdRequired, ok := stdOutputSchema["required"].([]string); ok {
		if outputRequired, ok := outputSchema["required"].([]string); ok {
			for _, reqField := range stdRequired {
				found := false
				for _, field := range outputRequired {
					if field == reqField {
						found = true
						break
					}
				}
				if !found {
					errors = append(errors, fmt.Sprintf("Output schema thiếu required field: %s", reqField))
				}
			}
		} else {
			errors = append(errors, "Output schema thiếu 'required' fields")
		}
	}

	return len(errors) == 0, errors
}
