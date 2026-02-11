package entity

type MetadataDatabaseOptions struct {
	URI            string `json:"uri" bson:"uri"`                         // URI kết nối database
	MaxPoolSize    int    `json:"max_pool_size" bson:"max_pool_size"`     // Số lượng connection tối đa
	MinPoolSize    int    `json:"min_pool_size" bson:"min_pool_size"`     // Số lượng connection tối thiểu
	ConnectTimeout int    `json:"connect_timeout" bson:"connect_timeout"` // Timeout khi kết nối
	SocketTimeout  int    `json:"socket_timeout" bson:"socket_timeout"`   // Timeout khi gửi nhận dữ liệu
}

// MetadataDatabase là struct cho metadata của database
type MetadataDatabase struct {
	Name        string                  `json:"name" bson:"name"`               // Tên database
	Description string                  `json:"description" bson:"description"` // Mô tả database
	Options     MetadataDatabaseOptions `json:"options" bson:"options"`         // Các tùy chọn của database
	Collections []MetadataCollection    `json:"collections" bson:"collections"` // Các collection của database
}
