// Package dto - DTO cho domain CRM (note).
package dto

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmNoteCreateInput dữ liệu tạo ghi chú mới.
type CrmNoteCreateInput struct {
	CustomerId     string `json:"customerId" validate:"required"` // unifiedId
	NoteText       string `json:"noteText" validate:"required"`
	NextAction     string `json:"nextAction,omitempty"`
	NextActionDate int64  `json:"nextActionDate,omitempty"`
}

// CrmNoteUpdateInput dữ liệu cập nhật ghi chú.
type CrmNoteUpdateInput struct {
	NoteText       string `json:"noteText,omitempty"`
	NextAction     string `json:"nextAction,omitempty"`
	NextActionDate int64  `json:"nextActionDate,omitempty"`
}

// CrmNoteResponse trả về ghi chú.
type CrmNoteResponse struct {
	ID               primitive.ObjectID `json:"id"`
	CustomerId       string             `json:"customerId"`
	NoteText         string             `json:"noteText"`
	NextAction       string             `json:"nextAction,omitempty"`
	NextActionDate   int64              `json:"nextActionDate,omitempty"`
	CreatedBy        primitive.ObjectID `json:"createdBy"`
	CreatedAt        int64              `json:"createdAt"`
	UpdatedAt        int64              `json:"updatedAt"`
}
