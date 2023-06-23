package service

import (
	"gateway/internal/orderbook/types"

	"github.com/Shopify/sarama"
)

type IOrderbookService interface {
	HandleConsume(message *sarama.ConsumerMessage)
	HandleConsumeBook(message *sarama.ConsumerMessage)
	HandleConsumeBookCancel(message *sarama.ConsumerMessage)
	HandleConsumeBookAgg(instrument string, order types.Order, isCancelledAll bool, cancelledBooks map[string]types.OrderbookMap)
	HandleConsumeUserChange(message *sarama.ConsumerMessage)
	HandleConsumeUserChangeCancel(message *sarama.ConsumerMessage)
	HandleConsumeTicker(message *sarama.ConsumerMessage)
	HandleConsumeTickerCancel(message *sarama.ConsumerMessage)
	Handle100msInterval(instrument string)
}
