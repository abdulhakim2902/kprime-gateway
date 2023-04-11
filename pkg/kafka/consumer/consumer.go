package consumer

import (
	"encoding/json"
	"fmt"
	engInt "gateway/internal/engine/service"
	obInt "gateway/internal/orderbook/service"
	"gateway/internal/ordermatch"
	"gateway/pkg/ws"
	"log"
	"os"

	"github.com/Shopify/sarama"
)

func KafkaConsumer(obSvc obInt.IOrderbookService, engSvc engInt.IEngineService) {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	brokers := []string{os.Getenv("KAFKA_BROKER")}
	topics := []string{"ORDER", "TRADE", "ORDERBOOK"}

	consumer, err := sarama.NewConsumer(brokers, config)
	if err != nil {
		log.Fatalf("Failed to create consumer: %s", err)
	}
	defer consumer.Close()

	for _, topic := range topics {
		partitionConsumer, err := consumer.ConsumePartition(topic, 0, sarama.OffsetNewest)
		if err != nil {
			log.Fatalf("Failed to create partition consumer for topic '%s': %s", topic, err)
		}

		go func(topic string) {
			for message := range partitionConsumer.Messages() {
				switch topic {
				case "ORDER":
					handleTopicOrder(message)
				case "TRADE":
					handleTopicTrade(message)
				case "ORDERBOOK":
					obSvc.HandleConsume(message)
				case "ENGINE":
					engSvc.HandleConsume(message)
				default:
					log.Printf("Unknown topic: %s", topic)
				}
			}
		}(topic)
	}

	select {}
}

func handleTopicOrder(message *sarama.ConsumerMessage) {
	fmt.Printf("Received message from ORDER: %s\n", string(message.Value))

	str := string(message.Value)
	var data map[string]interface{}
	err := json.Unmarshal([]byte(str), &data)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	// Send message to websocket
	userIDStr := fmt.Sprintf("%v", data["user_id"])
	ws.SendOrderMessage(userIDStr, data)
	ordermatch.OrderConfirmation(userIDStr, data)
}

func handleTopicTrade(message *sarama.ConsumerMessage) {
	fmt.Printf("Received message from TRADE: %s\n", string(message.Value))
}

func handleTopicOrderbook(message *sarama.ConsumerMessage) {
	fmt.Printf("Received message from ORDERBOOK: %s\n", string(message.Value))
}
