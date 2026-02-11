package basesvc

import (
	"context"
	"fmt"
	"meta_commerce/internal/common"
	"reflect"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RelationshipDefinition dinh nghia mot quan he tu struct tag
type RelationshipDefinition struct {
	CollectionName string
	FieldName      string
	ErrorMessage   string
	Optional       bool
	Cascade        bool
}

// ParseRelationshipTag phan tich struct tag relationship de lay cac dinh nghia quan he
func ParseRelationshipTag(structType reflect.Type) []RelationshipDefinition {
	var relationships []RelationshipDefinition
	if field, ok := structType.FieldByName("_Relationships"); ok {
		if tag := field.Tag.Get("relationship"); tag != "" {
			relationships = append(relationships, parseRelationshipTagValue(tag)...)
		}
	}
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.Name == "_Relationships" {
			continue
		}
		tag := field.Tag.Get("relationship")
		if tag == "" {
			continue
		}
		relationships = append(relationships, parseRelationshipTagValue(tag)...)
	}
	return relationships
}

func parseRelationshipTagValue(tagValue string) []RelationshipDefinition {
	var relationships []RelationshipDefinition
	parts := strings.Split(tagValue, "|")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		rel := RelationshipDefinition{}
		pairs := strings.Split(part, ",")
		for _, pair := range pairs {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			kv := strings.SplitN(pair, ":", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			switch key {
			case "collection":
				rel.CollectionName = value
			case "field":
				rel.FieldName = value
			case "message", "msg":
				rel.ErrorMessage = value
			case "optional":
				rel.Optional = value == "true" || value == "1"
			case "cascade":
				rel.Cascade = value == "true" || value == "1"
			}
		}
		if rel.CollectionName != "" && rel.FieldName != "" {
			if rel.ErrorMessage == "" {
				rel.ErrorMessage = fmt.Sprintf("Khong the xoa record vi co %%d record trong collection '%s' dang tham chieu toi.", rel.CollectionName)
			}
			relationships = append(relationships, rel)
		}
	}
	return relationships
}

// ValidateRelationships kiem tra cac quan he duoc dinh nghia trong struct tag
func ValidateRelationships(ctx context.Context, recordID primitive.ObjectID, structType reflect.Type) error {
	relationships := ParseRelationshipTag(structType)
	if len(relationships) == 0 {
		return nil
	}
	checks := make([]RelationshipCheck, 0, len(relationships))
	for _, rel := range relationships {
		if rel.Cascade {
			continue
		}
		checks = append(checks, RelationshipCheck{
			CollectionName: rel.CollectionName,
			FieldName:      rel.FieldName,
			ErrorMessage:   rel.ErrorMessage,
			Optional:       rel.Optional,
		})
	}
	if len(checks) > 0 {
		return CheckRelationshipExists(ctx, recordID, checks)
	}
	return nil
}

// ValidateRelationshipsFromValue kiem tra quan he tu mot gia tri struct
func ValidateRelationshipsFromValue(ctx context.Context, record interface{}, structType reflect.Type) error {
	recordID, ok := getIDFromModel(record)
	if !ok {
		return common.NewError(common.ErrCodeValidation, "Record khong co field ID", common.StatusBadRequest, nil)
	}
	if structType == nil {
		val := reflect.ValueOf(record)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		structType = val.Type()
	}
	return ValidateRelationships(ctx, recordID, structType)
}

// GetRelationshipDefinitions lay danh sach cac quan he duoc dinh nghia trong struct
func GetRelationshipDefinitions(structType reflect.Type) []RelationshipDefinition {
	return ParseRelationshipTag(structType)
}
