package service

import (
	"context"
	"encoding/json"
	"gateway/internal/deribit/model"
	"gateway/pkg/kafka/producer"
	"strings"
)

type deribitService struct {
	//
}

func NewDeribitService() IDeribitService {
	return &deribitService{}
}

func (svc deribitService) DeribitParseBuy(ctx context.Context, userId string, data model.DeribitRequest) (model.DeribitResponse, error) {
	_string := data.InstrumentName
	substring := strings.Split(_string, "-")

	buy := model.DeribitResponse{
		UserId:         userId,
		ClientId:       "",
		Underlying:     substring[0],
		ExpirationDate: substring[1],
		StrikePrice:    substring[2],
		Type:           data.Type,
		Side:           "buy",
		Price:          data.Price,
		Amount:         data.Amount,
	}

	_buy, err := json.Marshal(buy)
	if err != nil {
		panic(err)
	}
	//send to kafka
	producer.KafkaProducer(string(_buy), "NEWORDER")

	return buy, nil
}

func (svc deribitService) DeribitParseSell(ctx context.Context, userId string, data model.DeribitRequest) (model.DeribitResponse, error) {
	_string := data.InstrumentName
	substring := strings.Split(_string, "-")

	sell := model.DeribitResponse{
		UserId:         userId,
		ClientId:       "",
		Underlying:     substring[0],
		ExpirationDate: substring[1],
		StrikePrice:    substring[2],
		Type:           data.Type,
		Side:           "sell",
		Price:          data.Price,
		Amount:         data.Amount,
	}

	_sell, err := json.Marshal(sell)
	if err != nil {
		panic(err)
	}
	//send to kafka
	producer.KafkaProducer(string(_sell), "NEWORDER")

	return sell, nil
}

func (svc deribitService) DeribitParseEdit(ctx context.Context, userId string, data model.DeribitEditRequest) (model.DeribitResponse, error) {
	_string := data.InstrumentName
	substring := strings.Split(_string, "-")

	edit := model.DeribitResponse{
		UserId:         userId,
		ClientId:       "",
		Underlying:     substring[0],
		ExpirationDate: substring[1],
		StrikePrice:    substring[2],
		Type:           data.Type,
		Side:           "edit",
		Price:          data.Price,
		Amount:         data.Amount,
	}

	_edit, err := json.Marshal(edit)
	if err != nil {
		panic(err)
	}
	//send to kafka
	producer.KafkaProducer(string(_edit), "NEWORDER")

	return edit, nil
}

func (svc deribitService) DeribitParseCancel(ctx context.Context, userId string, data model.DeribitCancelRequest) (model.DeribitResponse, error) {
	_string := data.InstrumentName
	substring := strings.Split(_string, "-")

	cancel := model.DeribitResponse{
		UserId:         userId,
		ClientId:       "",
		Underlying:     substring[0],
		ExpirationDate: substring[1],
		StrikePrice:    substring[2],
		Type:           data.Type,
		Side:           "cancel",
		Price:          data.Price,
		Amount:         data.Amount,
	}

	_cancel, err := json.Marshal(cancel)
	if err != nil {
		panic(err)
	}
	//send to kafka
	producer.KafkaProducer(string(_cancel), "NEWORDER")

	return cancel, nil
}
