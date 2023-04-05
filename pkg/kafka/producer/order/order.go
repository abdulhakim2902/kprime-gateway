package order

import (
	"fmt"
	"os"

	"github.com/Shopify/sarama"
)

func ProduceOrder(obj string) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer([]string{os.Getenv("KAFKA_BROKER")}, config)
	if err != nil {
		panic(err)
	}
	defer producer.Close()

	topic := "NEWORDER"

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
