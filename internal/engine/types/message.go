package types

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type (
	EngineStatus string
	Type         string
	Side         string
	Contracts    string
	OrderStatus  string
	TradeStatus  string
)

type Message struct {
	Instrument string      `json:"instrument_name"`
	Bids       interface{} `json:"bids"`
	Asks       interface{} `json:"asks"`
}

type EngineResponse struct {
	Status  EngineStatus `json:"status,omitempty"`
	Order   *Order       `json:"order,omitempty"`
	Matches *Matches     `json:"matches,omitempty"`
}

type Matches struct {
	MakerOrders []*Order `json:"makerOrders"`
	TakerOrder  *Order   `json:"takerOrder"`
	Trades      []*Trade `json:"trades"`
}

type Order struct {
	ID           primitive.ObjectID `json:"id" bson:"_id"`
	UserID       string             `json:"userId" bson:"userId"`
	ClientID     string             `json:"clientId" bson:"clientId"`
	Underlying   string             `json:"underlying" bson:"underlying"`
	ExpiryDate   string             `json:"expiryDate" bson:"expiryDate"`
	StrikePrice  float64            `json:"strikePrice" bson:"strikePrice"`
	Type         Type               `json:"type" bson:"type"`
	Side         Side               `json:"side" bson:"side"`
	Price        float64            `json:"price" bson:"price"`
	Amount       float64            `json:"amount" bson:"amount"`
	FilledAmount float64            `json:"filledAmount" bson:"filledAmount"`
	Contracts    Contracts          `json:"contracts" bson:"contracts"`
	Status       OrderStatus        `json:"status" bson:"status"`
	Amendments   []interface{}      `json:"amendments" bson:"amendments"`
	CreatedAt    time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt    time.Time          `json:"updatedAt" bson:"updatedAt"`
	CancelledAt  *time.Time         `json:"cancelledAt" bson:"cancelledAt"`
}

type Trade struct {
	ID            primitive.ObjectID `json:"id" bson:"_id"`
	Underlying    string             `json:"underlying" bson:"underlying"`
	ExpiryDate    string             `json:"expiryDate" bson:"expiryDate"`
	StrikePrice   float64            `json:"strikePrice" bson:"strikePrice"`
	Side          Side               `json:"side" bson:"side"`
	Price         float64            `json:"price" bson:"price"`
	Amount        float64            `json:"amount" bson:"amount"`
	Status        TradeStatus        `json:"status" bson:"status"`
	Contracts     Contracts          `json:"contracts" bson:"contracts"`
	TakerID       string             `json:"takerId" bson:"takerId"`
	MakerID       string             `json:"makerId" bson:"makerId"`
	TakerClientID string             `json:"takerClientId" bson:"takerClientId"`
	MakerClientID string             `json:"makerClientId" bson:"makerClientId"`
	TakerOrderID  primitive.ObjectID `json:"takerOrderId" bson:"takerOrderId"`
	MakerOrderID  primitive.ObjectID `json:"makerOrderId" bson:"makerOrderId"`
	CreatedAt     time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt" bson:"updatedAt"`
}

const (
	ORDER_ADDED            EngineStatus = "ORDER_ADDED"
	ORDER_FILLED           EngineStatus = "ORDER_FILLED"
	ORDER_PARTIALLY_FILLED EngineStatus = "ORDER_PARTIALLY_FILLED"
	ORDER_CANCELLED        EngineStatus = "ORDER_CANCELLED"
	ORDER_AMENDED          EngineStatus = "ORDER_AMENDED"
)

const (
	LIMIT  Type = "LIMIT"
	MARKET Type = "MARKET"
)

const (
	OPEN           OrderStatus = "OPEN"
	PARTIAL_FILLED OrderStatus = "PARTIAL_FILLED"
	FILLED         OrderStatus = "FILLED"
	CANCELLED      OrderStatus = "CANCELLED"
	REJECTED       OrderStatus = "REJECTED"
)

const (
	PUT  Contracts = "PUT"
	CALL Contracts = "CALL"
)

const (
	BUY    Side = "BUY"
	SELL   Side = "SELL"
	EDIT   Side = "EDIT"
	CANCEL Side = "CANCEL"
)

const (
	SUCCESS TradeStatus = "SUCCESS"
	ADDED   TradeStatus = "ADDED"
)

type ErrorMessage struct {
	Error string `json:"error"`
}
