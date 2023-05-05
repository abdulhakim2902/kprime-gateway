package types

import (
	"time"

	"git.devucc.name/dependencies/utilities/models/order"
	"github.com/shopspring/decimal"
)

type Message struct {
	Instrument string      `json:"instrumentName"`
	Bids       interface{} `json:"bids"`
	Asks       interface{} `json:"asks"`
}

type Orderbook struct {
	InstrumentName string     `json:"instrumentName" bson:"instrumentName"`
	Bids           []*WsOrder `json:"bids" bson:"bids"`
	Asks           []*WsOrder `json:"asks" bson:"asks"`
}

type WsOrder struct {
	Price  float64 `json:"price" bson:"price"`
	Amount float64 `json:"amount" bson:"amount"`
}

type Order struct {
	order.Order          `bson:",inline"`
	InstrumentName       string `json:"instrumentName" bson:"instrumentName"`
	Symbol               string `json:"symbol" bson:"symbol"`
	SenderCompID         string `json:"sender_comp_id" bson:"sender_comp_id"`
	InsertTime           time.Time
	LastExecutedQuantity decimal.Decimal
	LastExecutedPrice    decimal.Decimal
}

type GetOrderBook struct {
	InstrumentName string  `json:"instrument_name"`
	Underlying     string  `json:"underlying"`
	ExpiryDate     string  `json:"expiryDate"`
	StrikePrice    float64 `json:"strikePrice"`
}
