package model

import "gorm.io/gorm"

type Role struct {
	gorm.Model
	ID   int    `json:"id"`
	Name string `json:"name"`
}
