package service

import (
	"github.com/Shopify/sarama"
)

type IEngineService interface {
	HandleConsume(message *sarama.ConsumerMessage)
	HandleConsumeQuote(message *sarama.ConsumerMessage)
}
