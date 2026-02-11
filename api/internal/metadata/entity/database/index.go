package entity

type MetadataIndexField struct {
	Name  string `json:"name" bson:"name"`   // Tên field
	Order string `json:"order" bson:"order"` // Thứ tự sắp xếp
}

type MetadataIndexOptions struct {
	Unique bool `json:"unique" bson:"unique"` // Unique index

}

// MetadataIndex là struct cho metadata của index
type MetadataIndex struct {
	Name        string               `json:"name" bson:"name"`               // Tên index
	Description string               `json:"description" bson:"description"` // Mô tả index
	Type        string               `json:"type" bson:"type"`               // Kiểu index
	Fields      []MetadataIndexField `json:"fields" bson:"fields"`           // Các field của index
	Options     MetadataIndexOptions `json:"options" bson:"options"`         // Các tùy chọn của index
}
