package producer

import (
	"fmt"
	"gateway/pkg/collector"
	"os"

	"github.com/Shopify/sarama"
)

func KafkaProducer(obj string, topic string) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer([]string{os.Getenv("KAFKA_BROKER")}, config)
	if err != nil {
		panic(err)
	}
	defer producer.Close()

	_topic := topic

	message := &sarama.ProducerMessage{
		Topic: _topic,
		Value: sarama.StringEncoder(obj),
	}

	partition, offset, err := producer.SendMessage(message)
	if err != nil {
		panic(err)
	}

	// Metrics
	go func() {
		collector.OutgoingKafkaCounter.Inc()
	}()

	fmt.Println("Kafka message sent to topic", topic, "partition", partition, "offset", offset)
}
