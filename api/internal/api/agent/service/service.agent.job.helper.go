package agentsvc

import (
	"fmt"
)

// JobRequiredFields là các trường bắt buộc cho job
var JobRequiredFields = []string{
	"name",
	"enabled",
	"schedule",
}

// ValidateJobStructure validate cấu trúc của một job
func ValidateJobStructure(job map[string]interface{}) error {
	for _, field := range JobRequiredFields {
		if _, ok := job[field]; !ok {
			return fmt.Errorf("job thiếu trường bắt buộc: %s", field)
		}
	}
	if name, ok := job["name"].(string); !ok || name == "" {
		return fmt.Errorf("job.name phải là string và không được để trống")
	}
	if _, ok := job["enabled"].(bool); !ok {
		return fmt.Errorf("job.enabled phải là boolean")
	}
	if schedule, ok := job["schedule"].(string); !ok || schedule == "" {
		return fmt.Errorf("job.schedule phải là string và không được để trống")
	}
	return nil
}

// ValidateJobsInConfigData validate tất cả jobs trong configData
func ValidateJobsInConfigData(configData map[string]interface{}) error {
	jobs, ok := configData["jobs"]
	if !ok {
		return nil
	}
	jobsArray, ok := jobs.([]interface{})
	if !ok {
		return fmt.Errorf("configData.jobs phải là array")
	}
	for i, jobInterface := range jobsArray {
		job, ok := jobInterface.(map[string]interface{})
		if !ok {
			return fmt.Errorf("job tại index %d phải là object", i)
		}
		if err := ValidateJobStructure(job); err != nil {
			return fmt.Errorf("job tại index %d không hợp lệ: %w", i, err)
		}
	}
	return nil
}
