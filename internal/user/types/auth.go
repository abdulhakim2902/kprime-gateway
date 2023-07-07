package types

type AuthRequest struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token" description:"Access Token to use for authentication"`
	ExpiresIn    int64  `json:"expires_in" description:"Token lifetime in seconds"`
	RefreshToken string `json:"refresh_token" description:"Can be used to request a new token (with a new lifetime)"`
	Scope        string `json:"scope" description:"Type of the access for assigned token"`
	TokenType    string `json:"token_type" description:"Authorization type, allowed value - bearer"`
}

type JwtClaim struct {
	UserID   string `json:"user_id"`
	UserRole string `json:"user_role"`
}
