package model

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type APILoginRequest struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}
