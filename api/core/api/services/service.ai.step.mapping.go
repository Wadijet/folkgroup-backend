package services

import (
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
)

// MapStepOutputToInput map output từ step này sang input của step tiếp theo
// Dựa trên standard schema để đảm bảo mapping chính xác
//
// Ví dụ:
// - GENERATE step output "text" → JUDGE step input "text"
// - Lớp AI: text in / text out. Lớp Logic: system tự map.
//
// Parameters:
//   - fromStepType: Loại step nguồn (GENERATE, JUDGE, STEP_GENERATION)
//   - toStepType: Loại step đích
//   - fromOutput: Output data từ step nguồn
//
// Returns:
//   - map[string]interface{}: Input data cho step đích
//   - error: Lỗi nếu có
func MapStepOutputToInput(fromStepType, toStepType string, fromOutput map[string]interface{}) (map[string]interface{}, error) {
	// Lấy standard schemas để biết cách map
	fromOutputSchema := models.GetStandardOutputSchema(fromStepType)
	toInputSchema := models.GetStandardInputSchema(toStepType)

	if fromOutputSchema == nil {
		return nil, fmt.Errorf("không tìm thấy standard output schema cho step type: %s", fromStepType)
	}
	if toInputSchema == nil {
		return nil, fmt.Errorf("không tìm thấy standard input schema cho step type: %s", toStepType)
	}

	result := make(map[string]interface{})

	// Mapping logic dựa trên từng cặp step types
	// Lớp AI: text in / text out. Lớp Logic: system tự map output → input.
	switch {
	// GENERATE → JUDGE: Map "text" (output schema: text, name, summary)
	case fromStepType == models.AIStepTypeGenerate && toStepType == models.AIStepTypeJudge:
		if text, ok := fromOutput["text"].(string); ok {
			result["text"] = text
		} else {
			return nil, fmt.Errorf("GENERATE step output thiếu field 'text' (bắt buộc cho JUDGE step)")
		}
		result["criteria"] = map[string]interface{}{
			"relevance":  10,
			"clarity":    10,
			"engagement": 10,
			"accuracy":   10,
		}
		if metadata, ok := fromOutput["metadata"].(map[string]interface{}); ok {
			result["metadata"] = metadata
		}

	// JUDGE → STEP_GENERATION: Map "text" hoặc metadata.feedback vào parentContext
	case fromStepType == models.AIStepTypeJudge && toStepType == models.AIStepTypeStepGeneration:
		if text, ok := fromOutput["text"].(string); ok {
			result["parentContext"] = map[string]interface{}{"content": text}
		} else if meta, ok := fromOutput["metadata"].(map[string]interface{}); ok {
			if feedback, ok := meta["feedback"].(string); ok {
				result["parentContext"] = map[string]interface{}{"content": feedback}
			}
		}

	// GENERATE → STEP_GENERATION: Map "text" vào parentContext
	case fromStepType == models.AIStepTypeGenerate && toStepType == models.AIStepTypeStepGeneration:
		if text, ok := fromOutput["text"].(string); ok {
			result["parentContext"] = map[string]interface{}{"content": text}
		} else {
			return nil, fmt.Errorf("GENERATE step output thiếu field 'text' (bắt buộc cho STEP_GENERATION step)")
		}

	default:
		// Generic mapping: Copy các fields có trong cả output schema và input schema
		toProps, _ := toInputSchema["properties"].(map[string]interface{})

		// Copy các fields từ output nếu field đó có trong input schema
		for fieldName := range toProps {
			if value, ok := fromOutput[fieldName]; ok {
				result[fieldName] = value
			}
		}
	}

	return result, nil
}

// ValidateStepOutputFormat kiểm tra xem output có đúng format theo standard schema không
// Parameters:
//   - stepType: Loại step
//   - output: Output data cần validate
//
// Returns:
//   - bool: true nếu hợp lệ
//   - []string: Danh sách lỗi nếu có
func ValidateStepOutputFormat(stepType string, output map[string]interface{}) (bool, []string) {
	var errors []string

	standardSchema := models.GetStandardOutputSchema(stepType)
	if standardSchema == nil {
		errors = append(errors, fmt.Sprintf("Không tìm thấy standard output schema cho step type: %s", stepType))
		return false, errors
	}

	// Kiểm tra required fields
	if required, ok := standardSchema["required"].([]string); ok {
		for _, reqField := range required {
			if _, exists := output[reqField]; !exists {
				errors = append(errors, fmt.Sprintf("Output thiếu required field: %s", reqField))
			}
		}
	}

	return len(errors) == 0, errors
}

// ValidateStepInputFormat kiểm tra xem input có đúng format theo standard schema không
// Parameters:
//   - stepType: Loại step
//   - input: Input data cần validate
//
// Returns:
//   - bool: true nếu hợp lệ
//   - []string: Danh sách lỗi nếu có
func ValidateStepInputFormat(stepType string, input map[string]interface{}) (bool, []string) {
	var errors []string

	standardSchema := models.GetStandardInputSchema(stepType)
	if standardSchema == nil {
		errors = append(errors, fmt.Sprintf("Không tìm thấy standard input schema cho step type: %s", stepType))
		return false, errors
	}

	// Kiểm tra required fields
	if required, ok := standardSchema["required"].([]string); ok {
		for _, reqField := range required {
			if _, exists := input[reqField]; !exists {
				errors = append(errors, fmt.Sprintf("Input thiếu required field: %s", reqField))
			}
		}
	}

	return len(errors) == 0, errors
}
