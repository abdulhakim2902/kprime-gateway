package model

import "time"

type Role struct {
	// gorm.Model
	ID        uint `gorm:"primarykey" json:"id"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string `json:"name"`
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
