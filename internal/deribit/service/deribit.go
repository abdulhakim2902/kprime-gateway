package service

import (
	"context"
	"encoding/json"
	"gateway/internal/deribit/model"
	"gateway/pkg/kafka/producer/order"
	"strings"
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

	_buy, err := json.Marshal(buy)
	if err != nil {
		panic(err)
	}
	order.ProduceOrder(string(_buy))

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

	_sell, err := json.Marshal(sell)
	if err != nil {
		panic(err)
	}
	order.ProduceOrder(string(_sell))

	return sell, nil
}
