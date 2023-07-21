package model

import (
	"github.com/Undercurrent-Technologies/kprime-utilities/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SubscribeChannelParameters struct {
	InstrumentName string `json:"instrument_name" example:"BTC-31JUN23-50000-C" validate:"required" description:"Instrument name"`
	Interval       int    `json:"interval" validate:"required" oneof:"raw,100ms,agg2" description:"Frequency of notifications"`
}

type SubscribeChannelResponse struct {
	Channel string  `json:"channel" description:"channel parameter"`
	Data    []Order `json:"data" description:"array of order"`
}

type Order struct {
	FilledAmount        float64            `json:"filled_amount" bson:"filledAmount" description:"Trade amount"`
	Amount              float64            `json:"amount" bson:"amount" description:"Trade amount"`
	InstrumentName      string             `json:"instrument_name" bson:"InstrumentName" description:"Unique instrument identifier"`
	Direction           types.Side         `json:"direction" bson:"direction" description:"Direction" oneof:"buy,sell"`
	Price               float64            `json:"price" bson:"price" description:"Price in base currency"`
	OrderId             primitive.ObjectID `json:"order_id" bson:"orderId" description:"Unique order identifier"`
	Replaced            bool               `json:"replaced" description:"true if the order was edited, otherwise false"`
	TimeInForce         types.TimeInForce  `json:"time_in_force" bson:"timeInForce" description:"Order time in force" oneof:"good_til_cancelled,good_til_day,fill_or_kill,immediate_or_cancel"`
	OrderType           types.Type         `json:"order_type" bson:"orderType" description:"Order type" oneof:"limit,market,stop_limit,stop_market"`
	OrderState          types.OrderStatus  `json:"order_state" bson:"orderState" description:"Order state" oneof:"open,filled,rejected,cancelled,untriggered"`
	MaxShow             float64            `json:"max_show" bson:"maxShow" description:"Maximum amount within an order to be shown to other customers"`
	PostOnly            bool               `json:"post_only" bson:"postOnly" description:"If true, the order is considered post-only"`
	ReduceOnly          bool               `json:"reduce_only" bson:"reduceOnly" description:"If true, the order is considered reduce-only which is intended to only reduce a current position"`
	Label               string             `json:"label,omitempty" bson:"label" description:"User defined label"`
	Usd                 float64            `json:"usd" bson:"usd" description:"Option price in USD"`
	CreationTimestamp   int64              `json:"creation_timestamp" bson:"creationTimestamp" description:"The timestamp (milliseconds since the Unix epoch)"`
	LastUpdateTimestamp int64              `json:"last_update_timestamp" bson:"lastUpdateTimestamp" description:"The timestamp (milliseconds since the Unix epoch)"`
	Api                 bool               `json:"api" bson:"api" description:"true if created with API"`
	AveragePrice        *float64           `json:"average_price" bson:"priceAvg" description:"Average fill price of the order"`
	CancelledReason     string             `json:"cancel_reason" bson:"cancelledReason" description:"Enumerated reason behind cancel"`
}
