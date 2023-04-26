package types

import (
	"time"

	"github.com/shopspring/decimal"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Type string
type Side string
type Contracts string
type OrderStatus string
type TimeInForce string

const (
	BUY        Side = "BUY"
	SELL       Side = "SELL"
	EDIT       Side = "EDIT"
	CANCEL     Side = "CANCEL"
	CANCEL_ALL Side = "CANCEL_ALL"
)

const (
	LIMIT  Type = "LIMIT"
	MARKET Type = "MARKET"
)

const (
	PUT  Contracts = "PUT"
	CALL Contracts = "CALL"
)

const (
	OPEN           OrderStatus = "OPEN"
	PARTIAL_FILLED OrderStatus = "PARTIAL_FILLED"
	FILLED         OrderStatus = "FILLED"
	CANCELLED      OrderStatus = "CANCELLED"
	REJECTED       OrderStatus = "REJECTED"
)

const (
	GOOD_TIL_CANCELLED  TimeInForce = "GOOD_TIL_CANCELLED"
	GOOD_TIL_DAY        TimeInForce = "GOOD_TIL_DAY"
	FILL_OR_KILL        TimeInForce = "FILL_OR_KILL"
	IMMEDIATE_OR_CANCEL TimeInForce = "IMMEDIATE_OR_CANCEL"
)

type Message struct {
	Instrument string      `json:"instrumentName"`
	Bids       interface{} `json:"bids"`
	Asks       interface{} `json:"asks"`
}

type Orderbook struct {
	InstrumentName string   `json:"instrumentName" bson:"instrumentName"`
	Bids           []*Order `json:"bids" bson:"bids"`
	Asks           []*Order `json:"asks" bson:"asks"`
}

type Order struct {
	ID                   primitive.ObjectID `json:"id" bson:"_id"`
	UserID               string             `json:"userId" bson:"userId"`
	ClientID             string             `json:"clientId" bson:"clientId"`
	ClOrdID              string             `json:"clOrdId,omitempty" bson:"clOrdId,omitempty"`
	InstrumentName       string             `json:"instrumentName" bson:"instrumentName"`
	Symbol               string             `json:"symbol" bson:"symbol"`
	SenderCompID         string             `json:"sender_comp_id" bson:"sender_comp_id"`
	TargetCompID         string             `json:"target_comp_id" bson:"target_comp_id"`
	Underlying           string             `json:"underlying" bson:"underlying"`
	ExpiryDate           string             `json:"expiryDate" bson:"expiryDate"`
	StrikePrice          float64            `json:"strikePrice" bson:"strikePrice"`
	Type                 Type               `json:"type" bson:"type"`
	Side                 Side               `json:"side" bson:"side"`
	Price                float64            `json:"price" bson:"price"`
	Amount               float64            `json:"amount" bson:"amount"`
	FilledAmount         float64            `json:"filledAmount" bson:"filledAmount"`
	Contracts            Contracts          `json:"contracts" bson:"contracts"`
	Status               OrderStatus        `json:"status,omitempty" bson:"status,omitempty"`
	TimeInForce          TimeInForce        `json:"timeInForce" bson:"timeInForce"`
	RejectedReason       int                `json:"reasonStatus" bson:"reasonStatus"`
	PMM                  []string           `json:"pmm,omitempty" bson:"pmm,omitempty"`
	Amendments           []interface{}      `json:"amendments,omitempty" bson:"amendments,omitempty"`
	CreatedAt            time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt            time.Time          `json:"updatedAt" bson:"updatedAt"`
	CancelledAt          time.Time          `json:"cancelledAt,omitempty" bson:"cancelledAt,omitempty"`
	insertTime           time.Time
	LastExecutedQuantity decimal.Decimal
	LastExecutedPrice    decimal.Decimal
}

type GetOrderBook struct {
	InstrumentName string  `json:"instrument_name"`
	Underlying     string  `json:"underlying"`
	ExpiryDate     string  `json:"expiryDate"`
	StrikePrice    float64 `json:"strikePrice"`
}
