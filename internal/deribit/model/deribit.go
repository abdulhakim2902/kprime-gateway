package model

type DeribitRequest struct {
	InstrumentName string  `json:"instrumentName" validate:"required"`
	Amount         float64 `json:"amount"`
	Type           string  `json:"type"`
	Price          float64 `json:"price"`
}

type DeribitCancelRequest struct {
	Id string `json:"id" validate:"required"`
}

type DeribitCancelResponse struct {
	Id   string `json:"id"`
	Side string `json:"side"`
}

type DeribitEditRequest struct {
	Id     string  `json:"id" validate:"required"`
	Side   string  `json:"side"`
	Price  float64 `json:"price"`
	Amount float64 `json:"amount"`
}

type DeribitEditResponse struct {
	Id     string  `json:"id"`
	Side   string  `json:"side"`
	Price  float64 `json:"price"`
	Amount float64 `json:"amount"`
}

type DeribitResponse struct {
	ID             string  `json:"id"`
	UserId         string  `json:"userId"`
	ClientId       string  `json:"clientId"`
	Underlying     string  `json:"underlying"`
	ExpirationDate string  `json:"expiryDate"`
	StrikePrice    float64 `json:"strikePrice"`
	Type           string  `json:"type"`
	Side           string  `json:"side"`
	Price          float64 `json:"price"`
	Amount         float64 `json:"amount"`
	Contracts      string  `json:"contracts"`
}
