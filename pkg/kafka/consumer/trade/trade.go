package trade

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Shopify/sarama"
)

func ConsumeTrade() {
	// Set up Kafka consumer configuration
	consumerConfig := sarama.NewConfig()
	consumerConfig.Consumer.Return.Errors = true
	consumerConfig.Consumer.Offsets.Initial = sarama.OffsetOldest

	// Create a new Kafka consumer instance
	consumer, err := sarama.NewConsumer([]string{"localhost:29092"}, consumerConfig)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := consumer.Close(); err != nil {
			panic(err)
		}
	}()

	// Subscribe to Kafka topic
	topic := "TRADE"
	partitions, err := consumer.Partitions(topic)
	if err != nil {
		panic(err)
	}
	for _, partition := range partitions {
		partitionConsumer, err := consumer.ConsumePartition(topic, partition, sarama.OffsetOldest)
		if err != nil {
			panic(err)
		}

		// Start consuming messages from Kafka topic partition
		go func() {
			for {
				select {
				case message := <-partitionConsumer.Messages():
					fmt.Printf("Kafka received message on topic %s, partition %d, offset %d:\n%s\n",
						message.Topic, message.Partition, message.Offset, string(message.Value))
				case err := <-partitionConsumer.Errors():
					fmt.Printf("Error: %v\n", err)
					return
				}
			}
		}()
	}

	// Set up channel for OS signals
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for OS signal to terminate program
	<-sigchan
	fmt.Println("Terminating program...")
}
