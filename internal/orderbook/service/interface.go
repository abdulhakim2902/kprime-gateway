package service

import (
	"github.com/Shopify/sarama"
)

type IOrderbookService interface {
	HandleConsume(message *sarama.ConsumerMessage)
	HandleConsumeBook(message *sarama.ConsumerMessage)
	HandleConsumeBookAgg(message *sarama.ConsumerMessage)
}
