package types

type AuthRequest struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

type JwtClaim struct {
	UserID string `json:"user_id"`
}
