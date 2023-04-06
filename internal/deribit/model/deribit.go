package model

type DeribitRequest struct {
	InstrumentName string  `json:"instrument_name"`
	Amount         float64 `json:"amount"`
	Type           string  `json:"type"`
	Price          float64 `json:"price"`
}

type DeribitEditRequest struct {
	OrderId        string  `json:"order_id" validate:"required"`
	InstrumentName string  `json:"instrument_name"`
	Amount         float64 `json:"amount" validate:"required"`
	Type           string  `json:"type"`
	Price          float64 `json:"price" validate:"required"`
}

type DeribitCancelRequest struct {
	OrderId        string  `json:"order_id" validate:"required"`
	InstrumentName string  `json:"instrument_name"`
	Amount         float64 `json:"amount"`
	Type           string  `json:"type"`
	Price          float64 `json:"price"`
}

type DeribitResponse struct {
	UserId         string  `json:"user_id"`
	ClientId       string  `json:"client_id"`
	Underlying     string  `json:"underlying"`
	ExpirationDate string  `json:"expiration_date"`
	StrikePrice    string  `json:"strike_price"`
	Type           string  `json:"type"`
	Side           string  `json:"side"`
	Price          float64 `json:"price"`
	Amount         float64 `json:"amount"`
}
