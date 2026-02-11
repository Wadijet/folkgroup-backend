// Package models - Token, JwtToken thuộc domain auth.
package models

import "github.com/dgrijalva/jwt-go"

// JwtToken chứa data được mã hóa trong JWT token.
type JwtToken struct {
	UserID       string `json:"userId"`
	Time         string `json:"time"`
	RandomNumber string `json:"randomNumber"`
	jwt.StandardClaims
}

// Token token theo hwid (mỗi thiết bị một token).
type Token struct {
	Hwid     string `json:"hwid" bson:"hwid,omitempty"`
	RoleID   string `json:"roleId" bson:"roleId,omitempty"`
	JwtToken string `json:"jwtToken,omitempty" bson:"jwtToken,omitempty"`
}
