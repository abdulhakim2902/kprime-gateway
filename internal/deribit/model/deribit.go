package model

type DeribitRequest struct {
	InstrumentName string  `json:"instrumentName" validate:"required"`
	Amount         float64 `json:"amount"`
	Type           string  `json:"type"`
	Price          float64 `json:"price"`
}

type DeribitEditRequest struct {
	OrderId        string  `json:"orderId" validate:"required"`
	InstrumentName string  `json:"instrumentName" validate:"required"`
	Amount         float64 `json:"amount" validate:"required"`
	Type           string  `json:"type"`
	Price          float64 `json:"price" validate:"required"`
}

type DeribitCancelRequest struct {
	OrderId        string  `json:"orderId" validate:"required"`
	InstrumentName string  `json:"instrumentName"`
	Amount         float64 `json:"amount"`
	Type           string  `json:"type"`
	Price          float64 `json:"price"`
}

type DeribitResponse struct {
	UserId         string  `json:"orderId"`
	ClientId       string  `json:"clientId"`
	Underlying     string  `json:"underlying"`
	ExpirationDate string  `json:"expiryDate"`
	StrikePrice    string  `json:"strikePrice"`
	Type           string  `json:"type"`
	Side           string  `json:"side"`
	Price          float64 `json:"price"`
	Amount         float64 `json:"amount"`
	Contracts      string  `json:"contracts"`
}
