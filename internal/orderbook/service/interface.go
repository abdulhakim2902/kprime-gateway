package service

import (
	"gateway/internal/orderbook/types"

	"github.com/Shopify/sarama"
)

type IOrderbookService interface {
	HandleConsume(message *sarama.ConsumerMessage)
	HandleConsumeBook(message *sarama.ConsumerMessage)
	HandleConsumeBookAgg(instrument string, order types.Order)
	Handle100msInterval(instrument string)
}
