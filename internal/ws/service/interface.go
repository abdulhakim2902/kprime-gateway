package service

import (
	"context"
	deribitModel "gateway/internal/deribit/model"
	"gateway/pkg/ws"

	"github.com/Shopify/sarama"
)

type IwsOrderbookService interface {
	Subscribe(c *ws.Client, instrument string)
	Unsubscribe(c *ws.Client)
}

type IwsOrderService interface {
	Subscribe(c *ws.Client, instrument string)
	Unsubscribe(c *ws.Client)
	HandleConsume(msg *sarama.ConsumerMessage, userId string)
	GetInstruments(ctx context.Context, request deribitModel.DeribitGetInstrumentsRequest) []deribitModel.DeribitGetInstrumentsResponse
}

type IwsTradeService interface {
	Subscribe(c *ws.Client, instrument string)
	Unsubscribe(c *ws.Client)
	HandleConsume(msg *sarama.ConsumerMessage)
	GetUserTradesByInstrument(
		ctx context.Context,
		userId string,
		request deribitModel.DeribitGetUserTradesByInstrumentsRequest,
	) []deribitModel.DeribitGetUserTradesByInstrumentsResponse
}
