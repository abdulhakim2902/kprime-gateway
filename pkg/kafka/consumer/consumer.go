package consumer

import (
	"encoding/json"
	"fmt"
	"gateway/pkg/collector"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"
	"log"
	"os"
	"strconv"
	"strings"
	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"

	engInt "gateway/internal/engine/service"
	_engineType "gateway/internal/engine/types"
	ordermatch "gateway/internal/fix-acceptor"
	obInt "gateway/internal/orderbook/service"
	"gateway/internal/repositories"

	"github.com/Shopify/sarama"

	oInt "gateway/internal/ws/service"
)

func KafkaConsumer(
	repo *repositories.OrderRepository,
	engSvc engInt.IEngineService,
	obSvc obInt.IOrderbookService,
	oSvc oInt.IwsOrderService,
	tradeSvc oInt.IwsTradeService,
	rawSvc oInt.IwsRawPriceService,
) {
	// Metrics
	go func() {
		collector.IncomingKafkaCounter.Inc()
	}()

	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	brokers := []string{os.Getenv("KAFKA_BROKER")}
	topics := []string{"ENGINE", "CANCELLED_ORDER", "PRICES", "ENGINE_SAVED", "CANCELLED_ORDER_SAVED"}

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
				case "ENGINE":
					go handleTopicOrder(oSvc, message)
					go engSvc.HandleConsume(message)
				case "ENGINE_SAVED":
					go engSvc.HandleConsumeQuote(message)
					go oSvc.HandleConsumeUserOrder(message)
					go tradeSvc.HandleConsumeUserTrades(message)
					go tradeSvc.HandleConsumeInstrumentTrades(message)
					go obSvc.HandleConsumeUserChange(message)
					go obSvc.HandleConsumeBook(message)
					go obSvc.HandleConsumeTicker(message)
				case "CANCELLED_ORDER":
					handleTopicCancelledOrders(message)
				case "CANCELLED_ORDER_SAVED":
					go obSvc.HandleConsumeUserChangeCancel(message)
					go oSvc.HandleConsumeUserOrderCancel(message)
					go obSvc.HandleConsumeBookCancel(message)
					go engSvc.HandleConsumeQuoteCancel(message)
					go obSvc.HandleConsumeTickerCancel(message)
				case "PRICES":
					go rawSvc.HandleConsume(message)
				default:
					log.Printf("Unknown topic: %s", topic)
				}
			}
		}(topic)
	}

	select {}
}

func handleTopicOrder(oSvc oInt.IwsOrderService, message *sarama.ConsumerMessage) {
	logs.Log.Info().Msg(fmt.Sprintf("Received message from ORDER: %s\n", string(message.Value)))

	str := string(message.Value)
	var data _engineType.EngineResponse
	err := json.Unmarshal([]byte(str), &data)
	if err != nil {
		logs.Log.Error().Err(err).Msg("Error parsing JSON")
		return
	}

	if data.Matches.TakerOrder == nil {
		return
	}
	// Send message to websocket

	// convert mongodb object id to string
	userIDStr := data.Matches.TakerOrder.UserID.Hex()

	// Get instrument name from TakerOrder
	takerOrder := data.Matches.TakerOrder;
	instrumentName := takerOrder.Underlying + "-" + takerOrder.ExpiryDate + "-" + fmt.Sprintf("%.0f", takerOrder.StrikePrice) + "-" + string(takerOrder.Contracts[0])
	go ordermatch.OrderConfirmation(userIDStr, *data.Matches.TakerOrder, instrumentName)

	// FIX Subscription
	go ordermatch.OnTradeHappens(data)

	// no need to use HandleConsume
	// userId := data.Matches.TakerOrder.UserID
	// oSvc.HandleConsume(message, userId)

	// Metrics
	clOrdID := fmt.Sprintf("%v", data.Matches.TakerOrder.ClOrdID)
	collector.EndKafkaDuration(userIDStr, clOrdID)
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

// func handleTopicCancelledOrdersSubcription(tradeSvc oInt.IwsTradeService, message *sarama.ConsumerMessage) {
// 	fmt.Printf("Received message from CANCELLED_ORDER_SAVED: %s\n", string(message.Value))

// 	str := string(message.Value)
// 	var data map[string]interface{}
// 	err := json.Unmarshal([]byte(str), &data)
// 	if err != nil {
// 		fmt.Println("Error parsing JSON:", err)
// 		return
// 	}
// 	dataArr := data["data"].([]interface{})
// 	for _, order := range dataArr {
// 		go obSvc.HandleConsumeUserChange(message)

// 		fmt.Printf("dt:%+v\n", order)
// 	}

// 	// userIDStr := data["query"].(map[string]interface{})["userId"].(string)
// 	// ClOrdID := data["query"].(map[string]interface{})["clOrdId"].(string)

// 	// ID, _ := strconv.ParseUint(ClOrdID, 0, 64)

// 	// connectionKey := utils.GetKeyFromIdUserID(ID, userIDStr)
// 	// _payload := len(dataArr)

// 	// // Metrics
// 	// collector.EndKafkaDuration(userIDStr, ClOrdID)

// 	// protocol.SendSuccessMsg(connectionKey, _payload)
// }

func handleTopicCancelledOrders(message *sarama.ConsumerMessage) {
	fmt.Printf("Received message from CANCELLED_ORDERS: %s\n", string(message.Value))

	str := string(message.Value)
	var data map[string]interface{}
	err := json.Unmarshal([]byte(str), &data)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}
	dataArr := data["data"].([]interface{})

	var userIDStr string
	var ClOrdID string

	if data["query"].(map[string]interface{})["userId"] != nil || data["query"].(map[string]interface{})["clOrdId"] != nil {
		userIDStr = data["query"].(map[string]interface{})["userId"].(string)
		ClOrdID = data["query"].(map[string]interface{})["clOrdId"].(string)
	} else if len(dataArr) > 0 {
		userIDStr = dataArr[0].(map[string]interface{})["userId"].(string)
		ClOrdID = dataArr[0].(map[string]interface{})["clOrdId"].(string)
	}

	ID, _ := strconv.ParseUint(ClOrdID, 0, 64)

	connectionKey := utils.GetKeyFromIdUserID(ID, userIDStr)
	_payload := len(dataArr)

	// Metrics
	collector.EndKafkaDuration(userIDStr, ClOrdID)

	protocol.SendSuccessMsg(connectionKey, _payload)
}
