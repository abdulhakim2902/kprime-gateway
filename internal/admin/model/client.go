package model

import "gorm.io/gorm"

type CreateClient struct {
	Name  string
	Email string
}

type Client struct {
	gorm.Model
	Name               string
	Email              string
	ClientId           string
	HashedClientSecret string
}
