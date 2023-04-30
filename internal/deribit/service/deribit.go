package service

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/deribit/model"
	"gateway/pkg/kafka/producer"
	"strconv"
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
	fmt.Println("data.InstrumentName")
	fmt.Println(data)
	fmt.Println(substring)
	_contracts := ""
	if substring[3] == "P" {
		_contracts = "PUT"
	} else {
		_contracts = "CALL"
	}

	strikePrice, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		panic(err)
	}

	_timeInForce := ""
	if data.TimeInForce == "" {
		_timeInForce = "good_til_canceled"
	} else {
		_timeInForce = data.TimeInForce
	}

	buy := model.DeribitResponse{
		UserId:         userId,
		ClientId:       "",
		Underlying:     substring[0],
		ExpirationDate: strings.ToUpper(substring[1]),
		StrikePrice:    strikePrice,
		Type:           data.Type,
		Side:           "BUY",
		ClOrdID:        data.ClOrdID,
		Price:          data.Price,
		Amount:         data.Amount,
		Contracts:      _contracts,
		TimeInForce:    _timeInForce,
	}

	_buy, err := json.Marshal(buy)
	if err != nil {
		panic(err)
	}
	//send to kafka
	producer.KafkaProducer(string(_buy), "NEW_ORDER")

	return buy, nil
}

func (svc deribitService) DeribitParseSell(ctx context.Context, userId string, data model.DeribitRequest) (model.DeribitResponse, error) {
	_string := data.InstrumentName
	substring := strings.Split(_string, "-")
	_contracts := ""
	if substring[3] == "P" {
		_contracts = "PUT"
	} else {
		_contracts = "CALL"
	}

	strikePrice, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		panic(err)
	}

	_timeInForce := ""
	if data.TimeInForce == "" {
		_timeInForce = "good_til_canceled"
	} else {
		_timeInForce = data.TimeInForce
	}

	sell := model.DeribitResponse{
		UserId:         userId,
		ClientId:       "",
		Underlying:     substring[0],
		ExpirationDate: strings.ToUpper(substring[1]),
		StrikePrice:    strikePrice,
		Type:           data.Type,
		Side:           "SELL",
		ClOrdID:        data.ClOrdID,
		Price:          data.Price,
		Amount:         data.Amount,
		Contracts:      _contracts,
		TimeInForce:    _timeInForce,
	}

	_sell, err := json.Marshal(sell)
	if err != nil {
		panic(err)
	}
	//send to kafka
	producer.KafkaProducer(string(_sell), "NEW_ORDER")

	return sell, nil
}

func (svc deribitService) DeribitParseEdit(ctx context.Context, userId string, data model.DeribitEditRequest) (model.DeribitEditResponse, error) {

	edit := model.DeribitEditResponse{
		Id:       data.Id,
		UserId:   userId,
		ClientId: "",
		Side:     "EDIT",
		ClOrdID:  data.ClOrdID,
		Price:    data.Price,
		Amount:   data.Amount,
	}

	_edit, err := json.Marshal(edit)
	if err != nil {
		panic(err)
	}
	//send to kafka
	producer.KafkaProducer(string(_edit), "NEW_ORDER")

	return edit, nil
}

func (svc deribitService) DeribitParseCancel(ctx context.Context, userId string, data model.DeribitCancelRequest) (model.DeribitCancelResponse, error) {
	cancel := model.DeribitCancelResponse{
		Id:       data.Id,
		UserId:   userId,
		ClientId: "",
		Side:     "CANCEL",
		ClOrdID:  data.ClOrdID,
	}

	_cancel, err := json.Marshal(cancel)
	if err != nil {
		panic(err)
	}
	//send to kafka
	producer.KafkaProducer(string(_cancel), "NEW_ORDER")

	return cancel, nil
}

func (svc deribitService) DeribitCancelByInstrument(ctx context.Context, userId string, data model.DeribitCancelByInstrumentRequest) (model.DeribitCancelByInstrumentResponse, error) {
	_string := data.InstrumentName
	substring := strings.Split(_string, "-")
	_contracts := ""
	if substring[3] == "P" {
		_contracts = "PUT"
	} else {
		_contracts = "CALL"
	}

	strikePrice, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		panic(err)
	}

	cancel := model.DeribitCancelByInstrumentResponse{
		UserId:         userId,
		ClientId:       "",
		Underlying:     substring[0],
		ExpirationDate: strings.ToUpper(substring[1]),
		StrikePrice:    strikePrice,
		Contracts:      _contracts,
		Side:           "CANCEL_ALL_BY_INSTRUMENT",
		ClOrdID:        data.ClOrdID,
	}

	_cancel, err := json.Marshal(cancel)
	if err != nil {
		panic(err)
	}
	//send to kafka
	producer.KafkaProducer(string(_cancel), "NEW_ORDER")

	return cancel, nil
}

func (svc deribitService) DeribitParseCancelAll(ctx context.Context, userId string, data model.DeribitCancelAllRequest) (model.DeribitCancelAllResponse, error) {
	cancel := model.DeribitCancelAllResponse{
		UserId:   userId,
		ClientId: "",
		Side:     "CANCEL_ALL",
		ClOrdID:  data.ClOrdID,
	}

	_cancel, err := json.Marshal(cancel)
	if err != nil {
		panic(err)
	}
	//send to kafka
	producer.KafkaProducer(string(_cancel), "NEW_ORDER")

	return cancel, nil
}

func (svc deribitService) DeribitGetUserTradesByInstrument(ctx context.Context, userId string, data model.DeribitCancelByInstrumentRequest) (model.DeribitCancelByInstrumentResponse, error) {
	_string := data.InstrumentName
	substring := strings.Split(_string, "-")
	_contracts := ""
	if substring[3] == "P" {
		_contracts = "PUT"
	} else {
		_contracts = "CALL"
	}

	strikePrice, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		panic(err)
	}

	cancel := model.DeribitCancelByInstrumentResponse{
		UserId:         userId,
		ClientId:       "",
		Underlying:     substring[0],
		ExpirationDate: strings.ToUpper(substring[1]),
		StrikePrice:    strikePrice,
		Contracts:      _contracts,
		Side:           "CANCEL_ALL_BY_INSTRUMENT",
		ClOrdID:        data.ClOrdID,
	}

	_cancel, err := json.Marshal(cancel)
	if err != nil {
		panic(err)
	}
	//send to kafka
	producer.KafkaProducer(string(_cancel), "NEW_ORDER")

	return cancel, nil
}
