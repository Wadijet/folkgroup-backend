// Package models chứa các kiểu dùng chung cho layer repository/base (kết quả phân trang, đếm).
package models

// PaginateResult đại diện cho kết quả phân trang
type PaginateResult[T any] struct {
	// Trang hiện tại
	Page int64 `json:"page" bson:"page"`
	// Số lượng mục trên mỗi trang
	Limit int64 `json:"limit" bson:"limit"`
	// Số lượng mục trong trang hiện tại
	ItemCount int64 `json:"itemCount" bson:"itemCount"`
	// Danh sách các mục
	Items []T `json:"items" bson:"items"`
	// Tổng số mục
	Total int64 `json:"total" bson:"total"`
	// Tổng số trang
	TotalPage int64 `json:"totalPage" bson:"totalPage"`
}

// CountResult đại diện cho kết quả đếm
type CountResult struct {
	// Tổng số lượng mục
	TotalCount int64 `json:"totalCount" bson:"totalCount"`
	// Số lượng mục trên mỗi trang
	Limit int64 `json:"limit" bson:"limit"`
	// Tổng số trang
	TotalPage int64 `json:"totalPage" bson:"totalPage"`
}
