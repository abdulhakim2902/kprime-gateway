package model

import (
	_orderbookType "gateway/internal/orderbook/types"

	"time"

	"git.devucc.name/dependencies/utilities/models/order"
	"git.devucc.name/dependencies/utilities/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EmptyParams struct{}

type RequestDto[T any] struct {
	Method  string `json:"method" form:"method"`
	Jsonrpc string `json:"jsonrpc" form:"jsonrpc"`
	Id      uint64 `json:"id" form:"id"`
	Params  T      `json:"params" form:"params"`
}

type RequestParams struct {
	Id             string            `json:"id" form:"id"`
	AccessToken    string            `json:"access_token" form:"access_token"`
	InstrumentName string            `json:"instrument_name" form:"instrument_name"`
	Amount         float64           `json:"amount" form:"amount"`
	Type           types.Type        `json:"type" form:"type"`
	Price          float64           `json:"price" form:"price"`
	TimeInForce    types.TimeInForce `json:"time_in_force" form:"time_in_force"`
	Label          string            `json:"label" form:"label"`
}

type ChannelParams struct {
	AccessToken string   `json:"access_token" form:"access_token"`
	Channels    []string `json:"channels" form:"channels"`
}

type GetInstrumentsParams struct {
	AccessToken string `json:"access_token" form:"access_token"`
	Currency    string `json:"currency" validate:"required" form:"currency"`
	Expired     bool   `json:"expired" form:"expired"`
}

type GetOrderBookParams struct {
	InstrumentName string `json:"instrument_name" validate:"required" form:"instrument_name"`
	Depth          int64  `json:"depth" form:"depth"`
}

type GetLastTradesByInstrumentParams struct {
	InstrumentName string    `json:"instrument_name" validate:"required" form:"instrument_name"`
	StartSeq       int64     `json:"start_seq" form:"start_seq"`
	EndSeq         int64     `json:"end_seq" form:"end_seq"`
	StartTimestamp time.Time `json:"start_timestamp" form:"start_timestamp"`
	EndTimestamp   time.Time `json:"end_timestamp" form:"end_timestamp"`
	Count          int64     `json:"count" form:"count"`
	Sorting        string    `json:"sorting" form:"sorting"`
}

type GetIndexPriceParams struct {
	IndexName string `json:"index_name" validate:"required" form:"index_name"`
}

type DeribitGetIndexPriceRequest struct {
	IndexName string `json:"index_name"`
}

type DeribitGetIndexPriceResponse struct {
	IndexPrice float64 `json:"index_price"`
}

type DeribitGetUserTradesByOrderValue struct {
	Amount         float64   `json:"amount"`
	Direction      string    `json:"direction"`
	InstrumentName string    `json:"instrument_name"`
	OrderId        string    `json:"order_id"`
	OrderType      string    `json:"order_type"`
	Price          float64   `json:"price"`
	State          string    `json:"state"`
	Timestamp      int64     `json:"timestamp"`
	TradeId        int32     `json:"trade_id"`
	Api            bool      `json:"api"`
	IndexPrice     float64   `json:"index_price"`
	Label          string    `json:"label"`
	TickDirection  int32     `json:"tick_direction"`
	TradeSeq       int32     `json:"trade_seq"`
	CreatedAt      time.Time `json:"createdAt"`
}

type DeribitGetUserTradesByOrderResponse struct {
	Trades []DeribitGetUserTradesByOrderValue `json:"trades"`
}

type BaseParams struct {
	AccessToken    string `json:"access_token" form:"access_token"`
	InstrumentName string `json:"instrument_name" validate:"required" form:"instrument_name"`
}

type GetUserTradesByInstrumentParams struct {
	BaseParams
	Count          int    `json:"count" form:"count"`
	StartTimestamp int64  `json:"start_timestamp" form:"start_timestamp"`
	EndTimestamp   int64  `json:"end_timestamp" form:"end_timestamp"`
	Sorting        string `json:"sorting" form:"sorting"`
}

type GetOpenOrdersByInstrumentParams struct {
	BaseParams
	Type string `json:"type" form:"type"`
}

type GetOrderHistoryByInstrumentParams struct {
	BaseParams
	Count           int  `json:"count" form:"count"`
	Offset          int  `json:"offset" form:"offset"`
	IncludeOld      bool `json:"include_old" form:"include_old"`
	IncludeUnfilled bool `json:"include_unfilled" form:"include_unfilled"`
}

type GetOrderStateParams struct {
	AccessToken string `json:"access_token"`
	OrderId     string `json:"order_id"`
}

type GetUserTradesByOrderParams struct {
	BaseParams
	OrderId string `json:"order_id" validate:"required" form:"order_id"`
	Sorting string `json:"sorting" form:"sorting"`
}

type DeribitRequest struct {
	ID             string            `json:"id"`
	ClientId       string            `json:"clientId"`
	InstrumentName string            `json:"instrument_name" validate:"required"`
	Amount         float64           `json:"amount"`
	Type           types.Type        `json:"type"`
	Price          float64           `json:"price"`
	ClOrdID        string            `json:"clOrdID"`
	TimeInForce    types.TimeInForce `json:"time_in_force"`
	Label          string            `json:"label"`
	Side           types.Side        `json:"side"`
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
	UserId         string          `json:"userId"`
	ClientId       string          `json:"clientId"`
	Underlying     string          `json:"underlying"`
	ExpirationDate string          `json:"expiryDate"`
	StrikePrice    float64         `json:"strikePrice"`
	Side           string          `json:"side"`
	Contracts      types.Contracts `json:"contracts"`
	ClOrdID        string          `json:"clOrdID"`
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
	ID             string            `json:"id,omitempty"`
	UserId         string            `json:"userId,omitempty"`
	ClientId       string            `json:"clientId,omitempty"`
	Underlying     string            `json:"underlying,omitempty"`
	ExpirationDate string            `json:"expiryDate,omitempty" bson:"expiryDate"`
	StrikePrice    float64           `json:"strikePrice,omitempty"`
	Type           types.Type        `json:"type,omitempty"`
	Side           types.Side        `json:"side,omitempty"`
	Price          float64           `json:"price,omitempty"`
	Amount         float64           `json:"amount,omitempty"`
	Contracts      types.Contracts   `json:"contracts,omitempty"`
	TimeInForce    types.TimeInForce `json:"timeInForce,omitempty"`
	ClOrdID        string            `json:"clOrdID,omitempty"`
	CreatedAt      time.Time         `json:"createdAt,omitempty"`
	Label          string            `json:"label,omitempty,omitempty"`
	FilledAmount   float64           `json:"filledAmount,omitempty"`
	Status         string            `json:"status,omitempty"`

	OrderExclusions []order.OrderExclusion `json:"order_exclusions,omitempty"`
	TypeInclusions  []order.TypeInclusions `json:"type_inclusions,omitempty"`
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

	OptionType         string  `json:"option_type"`
	SettlementCurrency string  `json:"settlement_currency"`
	Strike             float64 `json:"strike"`
}

type DeribitGetOrderBookRequest struct {
	InstrumentName string `json:"instrument_name"`
	Depth          int64  `json:"depth"`
}

type DeribitGetLastTradesByInstrumentRequest struct {
	InstrumentName string    `json:"instrument_name"`
	StartSeq       int64     `json:"start_seq"`
	EndSeq         int64     `json:"end_seq"`
	StartTimestamp time.Time `json:"start_timestamp"`
	EndTimestamp   time.Time `json:"end_timestamp"`
	Count          int64     `json:"count"`
	Sorting        string    `json:"sorting"`
}

type DeribitGetOrderBookResponse struct {
	Timestamp       int64                     `json:"timestamp"`
	Stats           OrderBookStats            `json:"stats"`
	Greeks          OrderBookGreek            `json:"greeks"`
	State           string                    `json:"state"`
	LastPrice       float64                   `json:"last_price"`
	Bids_iv         float64                   `json:"bid_iv"`
	Asks_iv         float64                   `json:"ask_iv"`
	InstrumentName  string                    `json:"instrument_name"`
	Bids            []*_orderbookType.WsOrder `json:"bids"`
	BestBidPrice    float64                   `json:"best_bid_price"`
	BestBidAmount   float64                   `json:"best_bid_amount"`
	BestAskPrice    float64                   `json:"best_ask_price"`
	BestAskAmount   float64                   `json:"best_ask_amount"`
	Asks            []*_orderbookType.WsOrder `json:"asks"`
	IndexPrice      *float64                  `json:"index_price"`
	SettlementPrice *float64                  `json:"settlement_price"`
	UnderlyingIndex *float64                  `json:"underlying_index"`
}

type DeribitGetLastTradesByInstrumentValue struct {
	Amount         float64   `json:"amount"`
	Direction      string    `json:"direction"`
	InstrumentName string    `json:"instrument_name"`
	Price          float64   `json:"price"`
	Timestamp      int64     `json:"timestamp"`
	TradeId        int32     `json:"trade_id"`
	Api            bool      `json:"api"`
	IndexPrice     float64   `json:"index_price"`
	TickDirection  int32     `json:"tick_direction"`
	TradeSeq       int32     `json:"trade_seq"`
	CreatedAt      time.Time `json:"created_at"`
}

type DeribitGetLastTradesByInstrumentResponse struct {
	Trades []DeribitGetLastTradesByInstrumentValue `json:"trades"`
}

type OrderBookStats struct {
	Volume      float64 `json:"volume"`
	PriceChange float64 `json:"price_change"`
	Low         float64 `json:"low"`
	High        float64 `json:"high"`
}

type OrderBookGreek struct {
	Delta float64 `json:"delta"`
	Vega  float64 `json:"vega"`
	Gamma float64 `json:"gamma"`
	Tetha float64 `json:"tetha"`
	Rho   float64 `json:"rho"`
}

type DeribitGetUserTradesByInstrumentsRequest struct {
	InstrumentName string `json:"instrument_name" validate:"required"`
	Count          int    `json:"count"`
	StartTimestamp int64  `json:"start_timestamp"`
	EndTimestamp   int64  `json:"end_timestamp"`
	Sorting        string `json:"sorting"`
}

type DeribitGetUserTradesByInstruments struct {
	TradeId        primitive.ObjectID `json:"trade_id" bson:"_id"`
	Amount         float64            `json:"amount" bson:"amount"`
	Direction      types.Side         `json:"direction" bson:"direction"`
	InstrumentName string             `json:"instrument_name" bson:"InstrumentName"`
	OrderId        primitive.ObjectID `json:"order_id" bson:"order_id"`
	OrderType      types.Type         `json:"order_type" bson:"order_type"`
	Price          float64            `json:"price" bson:"price"`
	State          types.OrderStatus  `json:"state" bson:"state"`
	Timestamp      int64              `json:"timestamp" bson:"timestamp"`
	Api            bool               `json:"api"`
	IndexPrice     float64            `json:"index_price" bson:"indexPrice"`
	Label          string             `json:"label,omitempty" bson:"label"`
	TickDirection  int                `json:"tick_direction" bson:"tickDirection"`
	TradeSequence  int                `json:"trade_seq" bson:"tradeSequence"`
}

type DeribitGetUserTradesByInstrumentsResponse struct {
	Trades  []*DeribitGetUserTradesByInstruments `json:"trades"`
	HasMore bool                                 `json:"has_more"`
}

type DeribitGetOpenOrdersByInstrumentRequest struct {
	InstrumentName string `json:"instrument_name" validate:"required"`
	Type           string `json:"type"`
}

type DeribitGetOpenOrdersByInstrumentResponse struct {
	FilledAmount   float64            `json:"filled_amount" bson:"filledAmount"`
	Amount         float64            `json:"amount" bson:"amount"`
	InstrumentName string             `json:"instrument_name" bson:"InstrumentName"`
	Direction      types.Side         `json:"direction" bson:"direction"`
	Price          float64            `json:"price" bson:"price"`
	OrderId        primitive.ObjectID `json:"order_id" bson:"orderId"`
	Replaced       bool               `json:"replaced"`
	TimeInForce    types.TimeInForce  `json:"time_in_force" bson:"timeInForce"`
	OrderType      types.Type         `json:"order_type" bson:"orderType"`
	OrderState     types.OrderStatus  `json:"order_state" bson:"orderState"`

	Label               string   `json:"label,omitempty" bson:"label"`
	Usd                 float64  `json:"usd" bson:"usd"`
	CreationTimestamp   int64    `json:"creation_timestamp" bson:"creationTimestamp"`
	LastUpdateTimestamp int64    `json:"last_update_timestamp" bson:"lastUpdateTimestamp"`
	Api                 bool     `json:"api" bson:"api"`
	AveragePrice        *float64 `json:"average_price" bson:"priceAvg"`
	CancelledReason     string   `json:"cancel_reason" bson:"cancelledReason"`
}

type DeribitGetOrderHistoryByInstrumentRequest struct {
	InstrumentName  string `json:"instrument_name" validate:"required"`
	Count           int    `json:"count"`
	Offset          int    `json:"offset"`
	IncludeOld      bool   `json:"include_old"`
	IncludeUnfilled bool   `json:"include_unfilled"`
}

type DeribitGetUserTradesByOrderRequest struct {
	OrderId string `json:"order_id " validate:"required"`
	Sorting string `json:"sorting "`
}

type DeribitGetOrderHistoryByInstrumentResponse struct {
	OrderState     string             `json:"order_state" bson:"orderState"`
	Amount         float64            `json:"amount" bson:"amount"`
	FilledAmount   float64            `json:"filled_amount" bson:"filledAmount"`
	InstrumentName string             `json:"instrument_name" bson:"InstrumentName"`
	Direction      string             `json:"direction" bson:"direction"`
	Price          float64            `json:"price" bson:"price"`
	OrderId        primitive.ObjectID `json:"order_id" bson:"orderId"`
	Replaced       bool               `json:"replaced" bson:"replaced"`
	OrderType      string             `json:"order_type" bson:"orderType"`
	TimeInForce    string             `json:"time_in_force" bson:"timeInForce"`

	Label               string   `json:"label,omitempty" bson:"label"`
	Usd                 float64  `json:"usd" bson:"usd"`
	CreationTimestamp   int64    `json:"creation_timestamp" bson:"creationTimestamp"`
	LastUpdateTimestamp int64    `json:"last_update_timestamp" bson:"lastUpdateTimestamp"`
	Api                 bool     `json:"api" bson:"api"`
	AveragePrice        *float64 `json:"average_price" bson:"priceAvg"`
	CancelledReason     string   `json:"cancel_reason" bson:"cancelledReason"`
}

type DeribitGetOrderStateRequest struct {
	OrderId string `json:"order_id"`
}

type DeribitGetOrderStateResponse struct {
	OrderState     string             `json:"order_state" bson:"orderState"`
	Amount         float64            `json:"amount" bson:"amount"`
	FilledAmount   float64            `json:"filled_amount" bson:"filledAmount"`
	InstrumentName string             `json:"instrument_name" bson:"InstrumentName"`
	Direction      string             `json:"direction" bson:"direction"`
	Price          float64            `json:"price" bson:"price"`
	OrderId        primitive.ObjectID `json:"order_id" bson:"orderId"`
	Replaced       bool               `json:"replaced" bson:"replaced"`
	OrderType      string             `json:"order_type" bson:"orderType"`
	TimeInForce    string             `json:"time_in_force" bson:"timeInForce"`

	Label               string   `json:"label,omitempty" bson:"label"`
	Usd                 float64  `json:"usd" bson:"usd"`
	CreationTimestamp   int64    `json:"creation_timestamp" bson:"creationTimestamp"`
	LastUpdateTimestamp int64    `json:"last_update_timestamp" bson:"lastUpdateTimestamp"`
	Api                 bool     `json:"api" bson:"api"`
	AveragePrice        *float64 `json:"average_price" bson:"priceAvg"`
	CancelledReason     string   `json:"cancel_reason" bson:"cancelledReason"`
}

type DeribitGetOrderStateByLabelRequest struct {
	AccessToken string `json:"access_token" form:"access_token"`
	Currency    string `json:"currency" form:"currency" validate:"required"`
	Label       string `json:"label" form:"label"`
	UserId      string
}

type DeribitGetOrderStateByLabelResponse struct {
	FilledAmount   float64            `json:"filled_amount" bson:"filledAmount"`
	Amount         float64            `json:"amount" bson:"amount"`
	Direction      types.Side         `json:"direction" bson:"direction"`
	InstrumentName string             `json:"instrument_name" bson:"InstrumentName"`
	Price          float64            `json:"price" bson:"price"`
	OrderId        primitive.ObjectID `json:"order_id" bson:"orderId"`
	Replaced       bool               `json:"replaced" bson:"replaced"`
	OrderType      string             `json:"order_type" bson:"orderType"`
	TimeInForce    string             `json:"time_in_force" bson:"timeInForce"`
	OrderState     types.OrderStatus  `json:"order_state" bson:"orderState"`

	Label               string   `json:"label,omitempty" bson:"label"`
	Usd                 float64  `json:"usd" bson:"usd"`
	CreationTimestamp   int64    `json:"creation_timestamp" bson:"creationTimestamp"`
	LastUpdateTimestamp int64    `json:"last_update_timestamp" bson:"lastUpdateTimestamp"`
	Api                 bool     `json:"api" bson:"api"`
	AveragePrice        *float64 `json:"average_price,omitempty" bson:"priceAvg"`
	CancelledReason     string   `json:"cancel_reason" bson:"cancelledReason"`
}

type DeliveryPricesParams struct {
	IndexName string `json:"index_name"`
	Offset    int    `json:"offset"`
	Count     int    `json:"count"`
}

type DeliveryPricesRequest struct {
	IndexName string `json:"index_name"`
	Offset    int    `json:"offset"`
	Count     int    `json:"count"`
}

type DeliveryPricesResponse struct {
	Date          string  `json:"date"`
	DeliveryPrice float64 `json:"delivery_price"`
	RecordsTotal  int     `json:"records_total"`
}
