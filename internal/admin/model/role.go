package model

import "gorm.io/gorm"

type Role struct {
	gorm.Model
	Name string `json:"name"`
	Data string `json:"data"`
}

type DeleteRole struct {
	ID uint `json:"id"`
}

type CreateRole struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type UpdateRole struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type DetailRole struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type ResponseRole struct {
	Response string
}
