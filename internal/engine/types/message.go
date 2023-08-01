package types

import (
	"time"

	_orderbookType "gateway/internal/orderbook/types"

	"github.com/Undercurrent-Technologies/kprime-utilities/models/price"
	"github.com/Undercurrent-Technologies/kprime-utilities/models/trade"
	"github.com/Undercurrent-Technologies/kprime-utilities/types"
	"github.com/Undercurrent-Technologies/kprime-utilities/types/validation_reason"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Message struct {
	Instrument string      `json:"instrument_name"`
	Bids       interface{} `json:"bids"`
	Asks       interface{} `json:"asks"`
}

type MessagePrices struct {
	RawPrice        []price.Price `json:"raw_prices"`
	SettlementPrice []price.Price `json:"settlement_prices"`
}

type EngineResponse struct {
	Status     types.EngineStatus                 `json:"status,omitempty"`
	Matches    *Matches                           `json:"matches,omitempty"`
	Validation validation_reason.ValidationReason `json:"validation"`
}

type Matches struct {
	MakerOrders []*_orderbookType.Order `json:"makerOrders"`
	TakerOrder  *_orderbookType.Order   `json:"takerOrder"`
	Trades      []*Trade                `json:"trades"`
}

type Trade struct {
	trade.Trade   `bson:",inline"`
	TakerID       string             `json:"takerId" bson:"takerId"`
	MakerID       string             `json:"makerId" bson:"makerId"`
	TakerClientID string             `json:"takerClientId" bson:"takerClientId"`
	MakerClientID string             `json:"makerClientId" bson:"makerClientId"`
	TakerOrderID  primitive.ObjectID `json:"takerOrderId" bson:"takerOrderId"`
	MakerOrderID  primitive.ObjectID `json:"makerOrderId" bson:"makerOrderId"`
	CreatedAt     time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt" bson:"updatedAt"`
}

type BuySellEditResponse struct {
	Order  BuySellEditCancelOrder `json:"order"`
	Trades []BuySellEditTrade     `json:"trades"`
}

type CancelResponse struct {
	Order BuySellEditCancelOrder `json:"order"`
}

type BuySellEditCancelOrder struct {
	OrderState          types.OrderStatus  `json:"order_state"`
	Usd                 float64            `json:"usd"`
	FilledAmount        float64            `json:"filled_amount"`
	InstrumentName      string             `json:"instrument_name"`
	Direction           types.Side         `json:"direction"`
	LastUpdateTimestamp int64              `json:"last_update_timestamp"`
	Price               float64            `json:"price"`
	Amount              float64            `json:"amount"`
	OrderId             primitive.ObjectID `json:"order_id"`
	Replaced            bool               `json:"replaced"`
	OrderType           types.Type         `json:"order_type"`
	TimeInForce         types.TimeInForce  `json:"time_in_force"`
	CreationTimestamp   int64              `json:"creation_timestamp"`
	Label               string             `json:"label,omitempty"`
	Api                 bool               `json:"api"`
	AveragePrice        float64            `json:"average_price,omitempty"`
	CancelReason        string             `json:"cancel_reason"`
	MaxShow             float64            `json:"max_show"`
	PostOnly            bool               `json:"post_only"`
	ReduceOnly          bool               `json:"reduce_only"`
}

type BuySellEditTrade struct {
	Advanced        string              `json:"advanced"`
	Amount          float64             `json:"amount"`
	Direction       types.Side          `json:"direction"`
	InstrumentName  string              `json:"instrument_name"`
	OrderId         primitive.ObjectID  `json:"order_id"`
	OrderType       types.Type          `json:"order_type"`
	Price           float64             `json:"price"`
	State           string              `json:"state"`
	Timestamp       int64               `json:"timestamp"`
	TradeId         primitive.ObjectID  `json:"trade_id"`
	Api             bool                `json:"api"`
	IndexPrice      float64             `json:"index_price"`
	Label           string              `json:"label,omitempty"`
	TickDirection   types.TickDirection `json:"tick_direction"`
	TradeSequence   int                 `json:"trade_seq"`
	UnderlyingPrice float64             `json:"underlying_price"`
	UnderlyingIndex string              `json:"underlying_index"`
	MarkPrice       float64             `json:"mark_price"`
}

type ErrorMessage struct {
	Error string `json:"error"`
}

type RawPrice struct {
	Id       primitive.ObjectID `json:"id" bson:"_id"`
	Price    float64            `json:"price" bson:"price"`
	Metadata Metadata           `json:"metadata" bson:"metadata"`
	Ts       time.Time          `json:"ts" bson:"ts"`
}

type SettlementPrice struct {
	ID       primitive.ObjectID `json:"id" bson:"_id"`
	Price    float64            `json:"price" bson:"price"`
	Metadata Metadata           `json:"metadata" bson:"metadata"`
	Ts       time.Time          `json:"ts" bson:"ts"`
}

type Metadata struct {
	Exchange string `json:"exchange" bson:"exchange"`
	Pair     string `json:"pair" bson:"pair"`
	Type     string `json:"type" bson:"type"`
}
