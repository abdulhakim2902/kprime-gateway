package model

import (
	"gateway/internal/admin/model"
	"time"
)

type CreateClient struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Client struct {
	ID                 uint       `gorm:"primarykey" json:"id"`
	Name               string     `json:"name"`
	Email              string     `json:"email"`
	ClientId           string     `json:"client_id"`
	Password           string     `json:"password"`
	Company            string     `json:"company"`
	HashedClientSecret string     `json:"hashed_client_secret"`
	RoleId             int        `json:"role_id"`
	Role               model.Role `gorm:"foreignKey:RoleId"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}
