package model

import "gorm.io/gorm"

type RegisterAdmin struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type Admin struct {
	gorm.Model
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	RoleId   int    `json:"role_id"`
	Role     Role   `gorm:"foreignKey:RoleId"`
}

type RequestKeyPassword struct {
	Id int `json:"id" validate:"required"`
}
