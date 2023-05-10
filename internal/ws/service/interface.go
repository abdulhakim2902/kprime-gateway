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
	Unsubscribe(c *ws.Client)
	UnsubscribeQuote(c *ws.Client)
	GetOrderBook(ctx context.Context, request deribitModel.DeribitGetOrderBookRequest) deribitModel.DeribitGetOrderBookResponse
	GetInitialDataQuote(order _orderbookTypes.GetOrderBook) _orderbookTypes.QuoteMessage
}

type IwsOrderService interface {
	Subscribe(c *ws.Client, instrument string)
	Unsubscribe(c *ws.Client)
	HandleConsume(msg *sarama.ConsumerMessage, userId string)
	GetInstruments(ctx context.Context, request deribitModel.DeribitGetInstrumentsRequest) []deribitModel.DeribitGetInstrumentsResponse
	GetOpenOrdersByInstrument(ctx context.Context, userId string, request deribitModel.DeribitGetOpenOrdersByInstrumentRequest) []deribitModel.DeribitGetOpenOrdersByInstrumentResponse
	GetGetOrderHistoryByInstrument(ctx context.Context, userId string, request deribitModel.DeribitGetOrderHistoryByInstrumentRequest) []deribitModel.DeribitGetOrderHistoryByInstrumentResponse
}

type IwsTradeService interface {
	Subscribe(c *ws.Client, instrument string)
	Unsubscribe(c *ws.Client)
	HandleConsume(msg *sarama.ConsumerMessage)
	GetUserTradesByInstrument(
		ctx context.Context,
		userId string,
		request deribitModel.DeribitGetUserTradesByInstrumentsRequest,
	) *deribitModel.DeribitGetUserTradesByInstrumentsResponse
}
