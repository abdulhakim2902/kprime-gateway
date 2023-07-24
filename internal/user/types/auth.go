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

type AuthParams struct {
	GrantType    string `json:"grant_type" validate:"required" oneof:"client_credentials,client_signature,refresh_token" description:"Method of authentication"`
	ClientID     string `json:"client_id" description:"Required for grant type client_credentials and client_signature"`
	ClientSecret string `json:"client_secret" description:"Required for grant type client_credentials"`
	RefreshToken string `json:"refresh_token" description:"Required for grant type refresh_token"`

	Signature string `json:"signature" description:"Required for grant type client_signature, it's a cryptographic signature calculated over provided fields using user secret key."`
	Timestamp string `json:"timestamp" description:"Required for grant type client_signature, provides time when request has been generated"`
	Nonce     string `json:"nonce" description:"Optional for grant type client_signature; delivers user generated initialization vector for the server token"`
	Data      string `json:"data" description:"Optional for grant type client_signature; contains any user specific value"`
}

type JwtClaim struct {
	UserID   string `json:"user_id"`
	UserRole string `json:"user_role"`
}
