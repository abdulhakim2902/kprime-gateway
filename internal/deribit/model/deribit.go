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

type DeribitGetInstrumentsRequest struct {
	Currency string `json:"currency" validate:"required"`
	Expired  bool   `json:"expired"`
}

type DeribitGetInstrumentsResponse struct {
	QuoteCurrency       string `json:"quote_currency"`
	PriceIndex          string `json:"price_index"`
	Kind                string `json:"kind"`
	IsActive            bool   `json:"is_active"`
	InstrumentName      string `json:"instrument_name"`
	ExpirationTimestamp int64  `json:"expiration_timestamp"`
	CreationTimestamp   int64  `json:"creation_timestamp"`
	ContractSize        uint64 `json:"contract_size"`
	BaseCurrency        string `json:"base_currency"`
}

type DeribitGetOrderBookRequest struct {
	InstrumentName string `json:"instrument_name"`
	Depth          int64  `json:"depth"`
}

type DeribitGetOrderBookResponse struct {
	Timestamp      int64          `json:"timestamp"`
	Stats          OrderBookStats `json:"stats"`
	State          string         `json:"state"`
	LastPrice      float64        `json:"last_price"`
	InstrumentName string         `json:"instrument_name"`
	Bids           [][]float64    `json:"bids"`
	BestBidPrice   float64        `json:"best_bid_price"`
	BestBidAmount  int64          `json:"best_bid_amount"`
	BestAskPrice   int64          `json:"best_ask_price"`
	BestAskAmount  int64          `json:"best_ask_amount"`
	Asks           []interface{}  `json:"asks"`
}

type OrderBookStats struct {
	Volume      float64 `json:"volume"`
	PriceChange float64 `json:"price_change"`
	PriceUSD    float64 `json:"volume_usd"`
	Low         float64 `json:"low"`
	High        float64 `json:"high"`
}
