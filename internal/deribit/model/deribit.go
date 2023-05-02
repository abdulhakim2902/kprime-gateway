package model

import (
	"gateway/internal/engine/types"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

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

type DeribitCancelAllRequest struct {
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

type DeribitCancelAllResponse struct {
	UserId   string `json:"userId"`
	ClientId string `json:"clientId"`
	Side     string `json:"side"`
	ClOrdID  string `json:"clOrdID"`
}

type DeribitCancelByInstrumentResponse struct {
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
	InstrumentName string `json:"instrument_name" validate:"required"`
	ClOrdID        string `json:"clOrdID"`
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

type DeribitGetUserTradesByInstrumentsRequest struct {
	InstrumentName string `json:"instrument_name" validate:"required"`
	Count          int    `json:"count"`
	StartTimestamp int64  `json:"start_timestamp"`
	EndTimestamp   int64  `json:"end_timestamp"`
	Sorting        string `json:"sorting"`
}

type DeribitGetUserTradesByInstruments struct {
	TradeId        string             `json:"trade_id" bson:"_id"`
	HasMore        string             `json:"has_more"`
	Amount         float64            `json:"amount" bson:"amount"`
	Direction      types.Side         `json:"direction" bson:"direction"`
	InstrumentName string             `json:"instrument_name"`
	OrderId        primitive.ObjectID `json:"order_id" bson:"order_id"`
	OrderType      types.Type         `json:"order_type" bson:"order_type"`
	Price          float64            `json:"price" bson:"price"`
	State          types.OrderStatus  `json:"state" bson:"state"`
	Timestamp      int64              `json:"timestamp"`
}

type DeribitGetUserTradesByInstrumentsResponse struct {
	Trades  []*DeribitGetUserTradesByInstruments `json:"trades"`
	HasMore bool                                 `json:"has_more"`
}
