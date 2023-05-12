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

type OrderbookSubscribe struct {
	InstrumentName string              `json:"instrumentName" bson:"instrumentName"`
	Bids           []*WsOrderSubscribe `json:"bids" bson:"bids"`
	Asks           []*WsOrderSubscribe `json:"asks" bson:"asks"`
}

type WsOrderSubscribe struct {
	Price  float64     `json:"price" bson:"price"`
	Amount float64     `json:"amount" bson:"amount"`
	Detail interface{} `json:"detail,omitempty" bson:"detail,omitempty"`
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

type QuoteResponse struct {
	Channel string      `json:"channel"`
	Data    interface{} `json:"data"`
}

type QuoteMessage struct {
	Timestamp     int64   `json:"timestamp"`
	Instrument    string  `json:"instrument_name"`
	BestAskAmount float64 `json:"best_ask_amount"`
	BestAskPrice  float64 `json:"best_ask_price"`
	BestBidAmount float64 `json:"best_bid_amount"`
	BestBidPrice  float64 `json:"best_bid_price"`
}

type BookData struct {
	Type           string          `json:"type"`
	Timestamp      int64           `json:"timestamp"`
	InstrumentName string          `json:"instrument_name"`
	ChangeId       int             `json:"change_id"`
	PrevChangeId   int             `json:"prev_change_id,omitempty"`
	Bids           [][]interface{} `json:"bids"`
	Asks           [][]interface{} `json:"asks"`
}

type Change struct {
	Id            int   `json:"id"`
	Timestamp     int64 `json:"timestamp"`
	TimestampPrev int64 `json:"timestamp_prev"`
}
