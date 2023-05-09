package types

import (
	"time"

	"git.devucc.name/dependencies/utilities/models/trade"
	"git.devucc.name/dependencies/utilities/types"
	"go.mongodb.org/mongo-driver/bson/primitive"

	_orderbookType "gateway/internal/orderbook/types"
)

type Message struct {
	Instrument string      `json:"instrument_name"`
	Bids       interface{} `json:"bids"`
	Asks       interface{} `json:"asks"`
}

type EngineResponse struct {
	Status  types.EngineStatus    `json:"status,omitempty"`
	Order   *_orderbookType.Order `json:"order,omitempty"`
	Matches *Matches              `json:"matches,omitempty"`
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
}

type BuySellEditTrade struct {
	Advanced       string             `json:"advanced"`
	Amount         float64            `json:"amount"`
	Direction      types.Side         `json:"direction"`
	InstrumentName string             `json:"instrument_name"`
	OrderId        primitive.ObjectID `json:"order_id"`
	OrderType      types.Type         `json:"order_type"`
	Price          float64            `json:"price"`
	State          types.TradeStatus  `json:"state"`
	Timestamp      int64              `json:"timestamp"`
	TradeId        primitive.ObjectID `json:"trade_id"`
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
