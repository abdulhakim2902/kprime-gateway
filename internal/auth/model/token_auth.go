package model

import "gorm.io/gorm"

type TokenAuth struct {
	gorm.Model
	UserID uint `json:"user_id"`
	AuthUUID string `json:"auth_uuid"`
}