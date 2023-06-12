package service

import (
	"context"

	deribitModel "gateway/internal/deribit/model"
	_orderbookTypes "gateway/internal/orderbook/types"
	"gateway/pkg/ws"

	"github.com/Shopify/sarama"
)

type IwsOrderbookService interface {
	Subscribe(c *ws.Client, instrument string)
	SubscribeQuote(c *ws.Client, instrument string)
	SubscribeBook(c *ws.Client, channel, instrument, interval string)
	SubscribeUserChange(c *ws.Client, instrument string, userId string)
	HandleConsumeUserChange(msg *sarama.ConsumerMessage)
	Unsubscribe(c *ws.Client)
	UnsubscribeQuote(c *ws.Client)
	UnsubscribeBook(c *ws.Client)
	GetOrderBook(ctx context.Context, request deribitModel.DeribitGetOrderBookRequest) deribitModel.DeribitGetOrderBookResponse
	GetLastTradesByInstrument(ctx context.Context, request deribitModel.DeribitGetLastTradesByInstrumentRequest) deribitModel.DeribitGetLastTradesByInstrumentResponse
	GetIndexPrice(ctx context.Context, request deribitModel.DeribitGetIndexPriceRequest) deribitModel.DeribitGetIndexPriceResponse
	GetOrderLatestTimestamp(o _orderbookTypes.GetOrderBook, after int64, isFilled bool) _orderbookTypes.Orderbook
	GetOrderLatestTimestampAgg(o _orderbookTypes.GetOrderBook, after int64) _orderbookTypes.Orderbook
	GetDataQuote(order _orderbookTypes.GetOrderBook) (_orderbookTypes.QuoteMessage, _orderbookTypes.Orderbook)
	GetDeliveryPrices(ctx context.Context, request deribitModel.DeliveryPricesRequest) deribitModel.DeliveryPricesResponse
}

type IwsOrderService interface {
	Subscribe(c *ws.Client, instrument string)
	Unsubscribe(c *ws.Client)
	SubscribeUserOrder(c *ws.Client, instrument string, userId string)
	HandleConsume(msg *sarama.ConsumerMessage, userId string)
	HandleConsumeUserOrder(msg *sarama.ConsumerMessage)
	GetInstruments(ctx context.Context, request deribitModel.DeribitGetInstrumentsRequest) []deribitModel.DeribitGetInstrumentsResponse
	GetOpenOrdersByInstrument(ctx context.Context, userId string, request deribitModel.DeribitGetOpenOrdersByInstrumentRequest) []deribitModel.DeribitGetOpenOrdersByInstrumentResponse
	GetGetOrderHistoryByInstrument(ctx context.Context, userId string, request deribitModel.DeribitGetOrderHistoryByInstrumentRequest) []deribitModel.DeribitGetOrderHistoryByInstrumentResponse
	GetOrderState(ctx context.Context, userId string, request deribitModel.DeribitGetOrderStateRequest) []deribitModel.DeribitGetOrderStateResponse
}

type IwsTradeService interface {
	Subscribe(c *ws.Client, instrument string)
	SubscribeUserTrades(c *ws.Client, instrument string, userId string)
	SubscribeTrades(c *ws.Client, instrument string)
	Unsubscribe(c *ws.Client)
	HandleConsume(msg *sarama.ConsumerMessage)
	HandleConsumeUserTrades(msg *sarama.ConsumerMessage)
	HandleConsumeInstrumentTrades(msg *sarama.ConsumerMessage)
	GetUserTradesByInstrument(
		ctx context.Context,
		userId string,
		request deribitModel.DeribitGetUserTradesByInstrumentsRequest,
	) *deribitModel.DeribitGetUserTradesByInstrumentsResponse
	GetUserTradesByOrder(ctx context.Context, userId string, InstrumentName string, request deribitModel.DeribitGetUserTradesByOrderRequest) deribitModel.DeribitGetUserTradesByOrderResponse
}

type IwsRawPriceService interface {
	Subscribe(c *ws.Client, instrument string)
	Unsubscribe(c *ws.Client)
	HandleConsume(msg *sarama.ConsumerMessage)
}

type IwsUserBalanceService interface {
	FetchUserBalance(currency string, userID string) deribitModel.GetAccountSummaryResult
}
