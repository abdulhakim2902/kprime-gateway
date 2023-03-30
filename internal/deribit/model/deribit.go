package model

type DeribitRequest struct {
	InstrumentName string `json:"instrument_name"`
	Amount         int    `json:"amount"`
	Type           string `json:"type"`
	Price          int    `json:"price"`
}

type DeribitResponse struct {
	UserId         string `json:"user_id"`
	ClientId       string `json:"client_id"`
	Underlying     string `json:"underlying"`
	ExpirationDate string `json:"expiration_date"`
	StrikePrice    string `json:"strike_price"`
	Type           string `json:"type"`
	Side           string `json:"side"`
	Price          int    `json:"price"`
	Amount         int    `json:"amount"`
}

// type User struct {
// 	Username string `json:"username" validate:"required"`
// 	Email string `json:"email" validate:"email,required"`
// }
