package consumer

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"gateway/internal/repositories"
	"gateway/pkg/ws"

	engInt "gateway/internal/engine/service"
	ordermatch "gateway/internal/fix-acceptor"
	obInt "gateway/internal/orderbook/service"

	"github.com/Shopify/sarama"

	oInt "gateway/internal/ws/service"
)

func KafkaConsumer(
	repo *repositories.OrderRepository,
	engSvc engInt.IEngineService,
	obSvc obInt.IOrderbookService,
	oSvc oInt.IwsOrderService,
	tradeSvc oInt.IwsTradeService,
) {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	brokers := []string{os.Getenv("KAFKA_BROKER")}
	topics := []string{"ORDER", "TRADE", "ORDERBOOK", "ENGINE"}

	fmt.Println(brokers)
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
					handleTopicOrder(oSvc, message)
				case "TRADE":
					handleTopicTrade(tradeSvc, message)
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

func handleTopicOrder(oSvc oInt.IwsOrderService, message *sarama.ConsumerMessage) {
	fmt.Printf("Received message from ORDER: %s\n", string(message.Value))

	str := string(message.Value)
	var data map[string]interface{}
	err := json.Unmarshal([]byte(str), &data)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	// Send message to websocket
	userIDStr := fmt.Sprintf("%v", data["userId"])
	ClOrdID := fmt.Sprintf("%v", data["clOrdId"])

	// Remove clOrdID
	delete(data, "clOrdId")

	ws.SendOrderMessage(userIDStr, data, ClOrdID)

	// symbol := strings.Split(data["underlying"].(string), "-")[0]
	var order ordermatch.Order
	err = json.Unmarshal([]byte(str), &order)
	if err != nil {
		fmt.Println("Error parsing order JSON:", err)
		return
	}
	fmt.Println(data)
	symbol := strings.Split(order.InstrumentName, "-")[0]
	ordermatch.OrderConfirmation(userIDStr, order, symbol)

	userId, ok := data["userId"].(string)
	if !ok {
		fmt.Println("Failed to convert interface{} to string")
		return
	}

	oSvc.HandleConsume(message, userId)
}

func handleTopicTrade(tradeSvc oInt.IwsTradeService, message *sarama.ConsumerMessage) {
	fmt.Printf("Received message from TRADE: %s\n", string(message.Value))

	tradeSvc.HandleConsume(message)
}

func handleTopicOrderbook(message *sarama.ConsumerMessage) {
	fmt.Printf("Received message from ORDERBOOK: %s\n", string(message.Value))

	str := string(message.Value)
	var data map[string]interface{}
	err := json.Unmarshal([]byte(str), &data)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}
	symbol := strings.Split(data["instrument_name"].(string), "-")[0]
	ordermatch.OnOrderboookUpdate(symbol, data)
}
