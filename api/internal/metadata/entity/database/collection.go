package entity

// MetadataCollection là struct cho metadata của collection
type MetadataCollection struct {
	Name        string          `json:"name" bson:"name"`               // Tên collection
	Description string          `json:"description" bson:"description"` // Mô tả collection
	Fields      []MetadataField `json:"fields" bson:"fields"`           // Các field của collection
	Indexes     []MetadataIndex `json:"indexes" bson:"indexes"`         // Các index của collection
}
