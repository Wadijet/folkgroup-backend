package entity

// MetadataField là struct cho metadata của field
type MetadataField struct {
	Name        string `json:"name" bson:"name"`               // Tên field
	Description string `json:"description" bson:"description"` // Mô tả field
	Type        string `json:"type" bson:"type"`               // Kiểu dữ liệu field
	Tag         string `json:"tag" bson:"tag"`                 // Tag của field
}
