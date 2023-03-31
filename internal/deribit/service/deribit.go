package service

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/deribit/model"
	"strings"

	"github.com/Shopify/sarama"
)

type deribitService struct {
	//
}

func NewDeribitService() IDeribitService {
	return &deribitService{}
}

func (svc deribitService) DeribitParseBuy(ctx context.Context, data model.DeribitRequest) (model.DeribitResponse, error) {
	_string := data.InstrumentName
	substring := strings.Split(_string, "-")

	buy := model.DeribitResponse{
		UserId:         "",
		ClientId:       "",
		Underlying:     substring[0],
		ExpirationDate: substring[1],
		StrikePrice:    substring[2],
		Type:           data.Type,
		Side:           "buy",
		Price:          data.Price,
		Amount:         data.Amount,
	}

	// Kafka Producer
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer([]string{"localhost:29092"}, config)
	if err != nil {
		panic(err)
	}
	defer producer.Close()

	topic := "deribit-buy"

	buyConverted, err := json.Marshal(buy)
	if err != nil {
		panic(err)
	}

	message := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(buyConverted),
	}

	partition, offset, err := producer.SendMessage(message)
	if err != nil {
		panic(err)
	}

	fmt.Println("Message sent to topic", topic, "partition", partition, "offset", offset)
	// End Kafka Producer

	return buy, nil
}

func (svc deribitService) DeribitParseSell(ctx context.Context, data model.DeribitRequest) (model.DeribitResponse, error) {
	_string := data.InstrumentName
	substring := strings.Split(_string, "-")

	sell := model.DeribitResponse{
		UserId:         "",
		ClientId:       "",
		Underlying:     substring[0],
		ExpirationDate: substring[1],
		StrikePrice:    substring[2],
		Type:           data.Type,
		Side:           "sell",
		Price:          data.Price,
		Amount:         data.Amount,
	}

	return sell, nil
}
