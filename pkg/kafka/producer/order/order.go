package order

import (
	"fmt"

	"github.com/Shopify/sarama"
)

func ProduceOrder(obj string) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer([]string{"localhost:29092"}, config)
	if err != nil {
		panic(err)
	}
	defer producer.Close()

	topic := "ORDER"

	message := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(obj),
	}

	partition, offset, err := producer.SendMessage(message)
	if err != nil {
		panic(err)
	}

	fmt.Println("Kafka message sent to topic", topic, "partition", partition, "offset", offset)
}
