package aisvc

import (
	"fmt"
	aimodels "meta_commerce/internal/api/ai/models"
)

// MapStepOutputToInput map output từ step này sang input của step tiếp theo
func MapStepOutputToInput(fromStepType, toStepType string, fromOutput map[string]interface{}) (map[string]interface{}, error) {
	fromOutputSchema := aimodels.GetStandardOutputSchema(fromStepType)
	toInputSchema := aimodels.GetStandardInputSchema(toStepType)
	if fromOutputSchema == nil {
		return nil, fmt.Errorf("không tìm thấy standard output schema cho step type: %s", fromStepType)
	}
	if toInputSchema == nil {
		return nil, fmt.Errorf("không tìm thấy standard input schema cho step type: %s", toStepType)
	}
	result := make(map[string]interface{})
	switch {
	case fromStepType == aimodels.AIStepTypeGenerate && toStepType == aimodels.AIStepTypeJudge:
		if text, ok := fromOutput["text"].(string); ok {
			result["text"] = text
		} else {
			return nil, fmt.Errorf("GENERATE step output thiếu field 'text' (bắt buộc cho JUDGE step)")
		}
		result["criteria"] = map[string]interface{}{"relevance": 10, "clarity": 10, "engagement": 10, "accuracy": 10}
		if metadata, ok := fromOutput["metadata"].(map[string]interface{}); ok {
			result["metadata"] = metadata
		}
	case fromStepType == aimodels.AIStepTypeJudge && toStepType == aimodels.AIStepTypeStepGeneration:
		if text, ok := fromOutput["text"].(string); ok {
			result["parentContext"] = map[string]interface{}{"content": text}
		} else if meta, ok := fromOutput["metadata"].(map[string]interface{}); ok {
			if feedback, ok := meta["feedback"].(string); ok {
				result["parentContext"] = map[string]interface{}{"content": feedback}
			}
		}
	case fromStepType == aimodels.AIStepTypeGenerate && toStepType == aimodels.AIStepTypeStepGeneration:
		if text, ok := fromOutput["text"].(string); ok {
			result["parentContext"] = map[string]interface{}{"content": text}
		} else {
			return nil, fmt.Errorf("GENERATE step output thiếu field 'text' (bắt buộc cho STEP_GENERATION step)")
		}
	default:
		toProps, _ := toInputSchema["properties"].(map[string]interface{})
		for fieldName := range toProps {
			if value, ok := fromOutput[fieldName]; ok {
				result[fieldName] = value
			}
		}
	}
	return result, nil
}

// ValidateStepOutputFormat kiểm tra output có đúng format theo standard schema không
func ValidateStepOutputFormat(stepType string, output map[string]interface{}) (bool, []string) {
	var errors []string
	standardSchema := aimodels.GetStandardOutputSchema(stepType)
	if standardSchema == nil {
		errors = append(errors, fmt.Sprintf("Không tìm thấy standard output schema cho step type: %s", stepType))
		return false, errors
	}
	if required, ok := standardSchema["required"].([]string); ok {
		for _, reqField := range required {
			if _, exists := output[reqField]; !exists {
				errors = append(errors, fmt.Sprintf("Output thiếu required field: %s", reqField))
			}
		}
	}
	return len(errors) == 0, errors
}

// ValidateStepInputFormat kiểm tra input có đúng format theo standard schema không
func ValidateStepInputFormat(stepType string, input map[string]interface{}) (bool, []string) {
	var errors []string
	standardSchema := aimodels.GetStandardInputSchema(stepType)
	if standardSchema == nil {
		errors = append(errors, fmt.Sprintf("Không tìm thấy standard input schema cho step type: %s", stepType))
		return false, errors
	}
	if required, ok := standardSchema["required"].([]string); ok {
		for _, reqField := range required {
			if _, exists := input[reqField]; !exists {
				errors = append(errors, fmt.Sprintf("Input thiếu required field: %s", reqField))
			}
		}
	}
	return len(errors) == 0, errors
}
