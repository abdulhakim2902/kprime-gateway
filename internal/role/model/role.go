package model

import "gorm.io/gorm"

type Role struct {
	gorm.Model
	Name string `json:"name"`
}

type DeleteClient struct {
	ID uint `json:"id"`
}

type CreateRole struct {
	Name string `json:"name"`
}

type ResponseRole struct {
	Response string
}
