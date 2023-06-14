package types

import (
	"time"

	"git.devucc.name/dependencies/utilities/types"

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

type OrderbookMap struct {
	InstrumentName string              `json:"instrumentName" bson:"instrumentName"`
	Bids           map[float64]WsOrder `json:"bids" bson:"bids"`
	Asks           map[float64]WsOrder `json:"asks" bson:"asks"`
}

type WsOrder struct {
	Price  float64 `json:"price" bson:"price"`
	Amount float64 `json:"amount" bson:"amount"`
}

type Count struct {
	Count int `json:"count" bson:"count"`
}

type Order struct {
	order.Order          `bson:",inline"`
	InstrumentName       string          `json:"instrumentName,omitempty" bson:"instrumentName"`
	Symbol               string          `json:"symbol,omitempty" bson:"symbol"`
	SenderCompID         string          `json:"sender_comp_id,omitempty" bson:"sender_comp_id"`
	InsertTime           time.Time       `json:"-"`
	LastExecutedQuantity decimal.Decimal `json:"-"`
	LastExecutedPrice    decimal.Decimal `json:"-"`
}

type CancelledOrder struct {
	Data      []*Order    `json:"data"`
	Query     interface{} `json:"query"`
	Nonce     int64       `json:"nonce"`
	CreatedAt time.Time   `json:"createdAt"`
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
	Id            int                `json:"id"`
	IdPrev        int                `json:"id_prev"`
	Timestamp     int64              `json:"timestamp"`
	TimestampPrev int64              `json:"timestamp_prev"`
	Bids          map[string]float64 `json:"bids"`
	Asks          map[string]float64 `json:"asks"`
	BidsAgg       map[string]float64 `json:"bids_agg"`
	AsksAgg       map[string]float64 `json:"asks_agg"`
	Bids100       map[string]float64 `json:"bids_100"`
	Asks100       map[string]float64 `json:"asks_100"`
}

type ChangeStruct struct {
	Id             int                     `json:"id"`
	IdPrev         int                     `json:"id_prev"`
	Status         types.OrderStatus       `json:"status"`
	Side           types.Side              `json:"side"`
	Price          float64                 `json:"price"`
	Amount         float64                 `json:"amount"`
	Amendments     []order.Amendment       `json:"amendments"`
	CancelledBooks map[string]OrderbookMap `json:"cancelled_books"`
}

type ChangeResponse struct {
	InstrumentName string        `json:"instrument_name"`
	Trades         []interface{} `json:"trades"`
	Orders         []interface{} `json:"orders"`
}

type PriceData struct {
	Timestamp int64   `json:"timestamp"`
	Price     float64 `json:"price"`
	IndexName string  `json:"index_name"`
}

type PriceResponse struct {
	Channel string    `json:"channel"`
	Data    PriceData `json:"data"`
}
