package types

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Side string
type Type string
type Contracts string
type OrderStatus string

const (
	BUY    Side = "BUY"
	SELL   Side = "SELL"
	EDIT   Side = "EDIT"
	CANCEL Side = "CANCEL"
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
)

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
	CreatedAt    time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt    time.Time          `json:"updatedAt" bson:"updatedAt"`
}
