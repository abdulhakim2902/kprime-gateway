package service

import (
	"context"
	"gateway/internal/deribit/model"
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

	return sell, nil
}
