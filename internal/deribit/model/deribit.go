package model

type DeribitRequest struct {
	InstrumentName string  `json:"instrument_name" validate:"required"`
	Amount         float64 `json:"amount"`
	Type           string  `json:"type"`
	Price          float64 `json:"price"`
	ClOrdID        string  `json:"clOrdID"`
	TimeInForce    string  `json:"time_in_force"`
}

type DeribitCancelRequest struct {
	Id      string `json:"id" validate:"required"`
	ClOrdID string `json:"clOrdID"`
}

type DeribitCancelResponse struct {
	Id       string `json:"id"`
	UserId   string `json:"userId"`
	ClientId string `json:"clientId"`
	Side     string `json:"side"`
	ClOrdID  string `json:"clOrdID"`
}

type DeribitCancelByInstrumentResponse struct {
	Id             string  `json:"id"`
	UserId         string  `json:"userId"`
	ClientId       string  `json:"clientId"`
	Underlying     string  `json:"underlying"`
	ExpirationDate string  `json:"expiryDate"`
	StrikePrice    float64 `json:"strikePrice"`
	Side           string  `json:"side"`
	Contracts      string  `json:"contracts"`
	ClOrdID        string  `json:"clOrdID"`
}

type DeribitCancelByInstrumentRequest struct {
	Id             string  `json:"id"`
	InstrumentName string  `json:"instrument_name" validate:"required"`
	Amount         float64 `json:"amount"`
	Type           string  `json:"type"`
	Price          float64 `json:"price"`
	ClOrdID        string  `json:"clOrdID"`
	TimeInForce    string  `json:"time_in_force"`
}

type DeribitEditRequest struct {
	Id      string  `json:"id" validate:"required"`
	Side    string  `json:"side"`
	Price   float64 `json:"price"`
	Amount  float64 `json:"amount"`
	ClOrdID string  `json:"clOrdID"`
}

type DeribitEditResponse struct {
	Id       string  `json:"id"`
	UserId   string  `json:"userId"`
	ClientId string  `json:"clientId"`
	Side     string  `json:"side"`
	Price    float64 `json:"price"`
	Amount   float64 `json:"amount"`
	ClOrdID  string  `json:"clOrdID"`
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
	TimeInForce    string  `json:"timeInForce"`
	ClOrdID        string  `json:"clOrdID"`
}
