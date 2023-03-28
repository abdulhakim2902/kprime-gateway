package model

import "time"

type CreateClient struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Client struct {
	ID                 uint      `gorm:"primarykey" json:"id"`
	Name               string    `json:"name" gorm:"<-"`
	Email              string    `json:"email" gorm:"<-"`
	ClientId           string    `json:"client_id" gorm:"<-"`
	Password           string    `json:"password" gorm:"<-"`
	HashedClientSecret string    `json:"hashed_client_secret" gorm:"<-"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
