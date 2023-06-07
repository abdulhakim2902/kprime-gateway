package kafka

import (
	"context"
	"encoding/json"

	"git.devucc.name/dependencies/utilities/commons/log"
	"git.devucc.name/dependencies/utilities/models/order"
	"github.com/segmentio/kafka-go"
)

var logger = log.Logger
var groupID = "gateway-group"

func InitConsumer(url string) *kafka.Reader {
	config := kafka.ReaderConfig{
		Brokers: []string{url},
		GroupID: groupID,
		Topic:   "NEW_ORDER",
	}

	return kafka.NewReader(config)
}

func (k *Kafka) Subscribe(cb func(order *order.Order)) {
	go func() {
		for {
			m, e := k.reader.ReadMessage(context.Background())
			if e != nil {
				logger.Errorf("Failed to read message!")
				continue
			}

			logger.Infof("Received messages from %v: %v", m.Topic, string(m.Value))

			o := &order.Order{}
			err := json.Unmarshal(m.Value, o)
			if err != nil {
				logger.Errorf("Failed to parse message!")
				continue
			}
			go cb(o)
		}
	}()
}
