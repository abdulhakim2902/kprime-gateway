package model

import (
	"time"
)

type Permission struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Name      string    `json:"name"`
	Abilities string    `json:"abilities"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreatePermission struct {
	Name      string `json:"name"`
	Abilities string `json:"abilities"`
}

type UpdatePermission struct {
	Name      string `json:"name"`
	Abilities string `json:"abilities"`
}

type ResponsePermission struct {
	Response string
}
