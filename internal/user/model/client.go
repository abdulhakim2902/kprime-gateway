package model

import (
	"gateway/internal/admin/model"
	"time"
)

type CreateClient struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Company  string `json:"company"`
	Password string `json:"password"`
	RoleId   int    `json:"role_id"`
}

type Client struct {
	ID        uint       `gorm:"primarykey" json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Password  string     `json:"pasword"`
	APIKey    string     `json:"api_key"`
	Company   string     `json:"company"`
	APISecret string     `json:"api_secret"`
	RoleId    int        `json:"role_id"`
	Role      model.Role `gorm:"foreignKey:RoleId"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type APIKeys struct {
	Password  string
	APIKey    string
	APISecret string
}
