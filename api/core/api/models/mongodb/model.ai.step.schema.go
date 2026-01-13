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
//   + Tạo candidates, scores, rankings, etc. từ parsed output
//
// QUAN TRỌNG VỀ STEP INPUT/OUTPUT:
// - Step Input: Dữ liệu đầu vào cho step (layerId, context, etc.) - dùng để generate prompt
// - Step Output: Dữ liệu đầu ra của step (candidates[], scores[], etc.) - bao gồm parsed AI output + system metadata
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

// GetStandardGenerateInputSchema trả về input schema chuẩn cho GENERATE step
// 
// LƯU Ý:
// - Step Input: Dữ liệu đầu vào (layerId, context, etc.) - dùng để generate prompt
// - AI Input (prompt): CHỈ là TEXT - được generate từ step input này
// - AI Output (response): CHỈ là TEXT - raw response từ AI
// - System sẽ parse AI response text → candidates[] và bổ sung metadata (timestamps, tokens, etc.)
// - Step Output: candidates[] + metadata (generatedAt, model, tokens) - để JUDGE step sử dụng
func GetStandardGenerateInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"required": []string{"layerId", "layerName", "targetAudience"},
		"properties": map[string]interface{}{
			"layerId": map[string]interface{}{
				"type":        "string",
				"description": "ID của layer cần generate content",
			},
			"layerName": map[string]interface{}{
				"type":        "string",
				"description": "Tên của layer",
			},
			"layerDescription": map[string]interface{}{
				"type":        "string",
				"description": "Mô tả của layer",
			},
			"targetAudience": map[string]interface{}{
				"type":        "string",
				"description": "Đối tượng mục tiêu",
				"enum":        []string{"B2B", "B2C", "B2B2C"},
			},
			"context": map[string]interface{}{
				"type":        "object",
				"description": "Context bổ sung cho việc generate",
				"properties": map[string]interface{}{
					"industry": map[string]interface{}{
						"type":        "string",
						"description": "Ngành nghề",
					},
					"productType": map[string]interface{}{
						"type":        "string",
						"description": "Loại sản phẩm",
					},
					"tone": map[string]interface{}{
						"type":        "string",
						"description": "Tone của content",
						"enum":        []string{"professional", "casual", "friendly", "formal"},
					},
				},
			},
			"numberOfCandidates": map[string]interface{}{
				"type":        "integer",
				"description": "Số lượng candidates cần generate",
				"minimum":     1,
				"maximum":     10,
				"default":     3,
			},
		},
	}
}

// GetStandardGenerateOutputSchema trả về output schema chuẩn cho GENERATE step
// 
// LƯU Ý:
// - AI Output (response): CHỈ là TEXT - raw response từ AI API
// - System sẽ parse text này → structured data (candidates[])
// - System tự bổ sung: generatedAt, model, tokens (không phải từ AI)
// 
// QUAN TRỌNG: Field "candidates" là bắt buộc và sẽ được sử dụng làm input cho JUDGE step
func GetStandardGenerateOutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"required": []string{"candidates", "generatedAt"},
		"properties": map[string]interface{}{
			"candidates": map[string]interface{}{
				"type":        "array",
				"description": "Danh sách các content candidates đã được generate (BẮT BUỘC cho JUDGE step)",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"candidateId": map[string]interface{}{
							"type":        "string",
							"description": "ID của candidate (tự động generate hoặc từ system)",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "Nội dung của candidate",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Tiêu đề của candidate",
						},
						"summary": map[string]interface{}{
							"type":        "string",
							"description": "Tóm tắt của candidate",
						},
						"metadata": map[string]interface{}{
							"type":        "object",
							"description": "Metadata bổ sung",
							"properties": map[string]interface{}{
								"wordCount": map[string]interface{}{
									"type": "integer",
								},
								"language": map[string]interface{}{
									"type": "string",
								},
								"tone": map[string]interface{}{
									"type": "string",
								},
							},
						},
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
				"type":        "object",
				"description": "Thông tin về tokens đã sử dụng",
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

// GetStandardJudgeInputSchema trả về input schema chuẩn cho JUDGE step
// 
// LƯU Ý:
// - Step Input: candidates[] từ GENERATE step + criteria + context
// - AI Input (prompt): CHỈ là TEXT - được generate từ step input này
// - AI Output (response): CHỈ là TEXT - raw response từ AI
// - System sẽ parse text này → structured data (scores[], rankings[])
// - System tự bổ sung: judgedAt (không phải từ AI)
// 
// QUAN TRỌNG: Field "candidates" phải match với output "candidates" từ GENERATE step
func GetStandardJudgeInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"required": []string{"candidates", "criteria"},
		"properties": map[string]interface{}{
			"candidates": map[string]interface{}{
				"type":        "array",
				"description": "Danh sách candidates cần đánh giá (từ output của GENERATE step)",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"candidateId": map[string]interface{}{
							"type":        "string",
							"description": "ID của candidate",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "Nội dung của candidate",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Tiêu đề của candidate",
						},
						"summary": map[string]interface{}{
							"type":        "string",
							"description": "Tóm tắt của candidate (optional)",
						},
					},
				},
			},
			"criteria": map[string]interface{}{
				"type":        "object",
				"description": "Tiêu chí đánh giá",
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
			"context": map[string]interface{}{
				"type":        "object",
				"description": "Context để đánh giá",
				"properties": map[string]interface{}{
					"targetAudience": map[string]interface{}{
						"type":        "string",
						"description": "Đối tượng mục tiêu",
					},
					"industry": map[string]interface{}{
						"type":        "string",
						"description": "Ngành nghề",
					},
				},
			},
		},
	}
}

// GetStandardJudgeOutputSchema trả về output schema chuẩn cho JUDGE step
// 
// LƯU Ý:
// - AI Output (response): CHỈ là TEXT - raw response từ AI API
// - System sẽ parse text này → structured data (scores[], rankings[], bestCandidate)
// - System tự bổ sung: judgedAt (không phải từ AI)
func GetStandardJudgeOutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"required": []string{"scores", "rankings", "judgedAt"},
		"properties": map[string]interface{}{
			"scores": map[string]interface{}{
				"type":        "array",
				"description": "Điểm số của từng candidate",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"candidateId": map[string]interface{}{
							"type":        "string",
							"description": "ID của candidate",
						},
						"overallScore": map[string]interface{}{
							"type":        "number",
							"description": "Điểm tổng thể (0-10)",
						},
						"criteriaScores": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"relevance": map[string]interface{}{
									"type": "number",
								},
								"clarity": map[string]interface{}{
									"type": "number",
								},
								"engagement": map[string]interface{}{
									"type": "number",
								},
								"accuracy": map[string]interface{}{
									"type": "number",
								},
							},
						},
						"feedback": map[string]interface{}{
							"type":        "string",
							"description": "Nhận xét về candidate",
						},
					},
				},
			},
			"rankings": map[string]interface{}{
				"type":        "array",
				"description": "Xếp hạng các candidates theo điểm số",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"rank": map[string]interface{}{
							"type": "integer",
						},
						"candidateId": map[string]interface{}{
							"type": "string",
						},
						"score": map[string]interface{}{
							"type": "number",
						},
					},
				},
			},
			"bestCandidate": map[string]interface{}{
				"type":        "object",
				"description": "Candidate tốt nhất",
				"properties": map[string]interface{}{
					"candidateId": map[string]interface{}{
						"type": "string",
					},
					"score": map[string]interface{}{
						"type": "number",
					},
					"reason": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"judgedAt": map[string]interface{}{
				"type":        "string",
				"format":      "date-time",
				"description": "Thời gian đánh giá",
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
		"type": "object",
		"required": []string{"parentContext", "requirements", "targetLevel"},
		"properties": map[string]interface{}{
			"parentContext": map[string]interface{}{
				"type":        "object",
				"description": "Context từ parent layer/step",
				"properties": map[string]interface{}{
					"layerId": map[string]interface{}{
						"type":        "string",
						"description": "ID của parent layer",
					},
					"layerName": map[string]interface{}{
						"type": "string",
					},
					"layerType": map[string]interface{}{
						"type": "string",
						"enum": []string{"L1", "L2", "L3", "L4", "L5", "L6", "L7", "L8"},
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Nội dung của parent layer",
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
		"type": "object",
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
				"type":        "object",
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
