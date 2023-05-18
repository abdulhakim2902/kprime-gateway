package service

import (
	"context"
	"encoding/json"
	"gateway/internal/deribit/model"
	"gateway/pkg/kafka/producer"
	"strconv"
	"strings"

	"git.devucc.name/dependencies/utilities/types"
)

type deribitService struct {
	//
}

func NewDeribitService() IDeribitService {
	return &deribitService{}
}

func (svc deribitService) DeribitRequest(
	ctx context.Context,
	userId string,
	data model.DeribitRequest,
) (model.DeribitResponse, error) {
	substring := strings.Split(data.InstrumentName, "-")

	var _underlying, _expDate string
	var _contracts types.Contracts

	strikePrice := new(float64)
	if len(substring) == 4 {
		_underlying = substring[0]
		_expDate = strings.ToUpper(substring[1])

		strike, err := strconv.ParseFloat(substring[2], 64)
		if err != nil {
			panic(err)
		}
		*strikePrice = strike

		if substring[3] == "P" {
			_contracts = types.PUT
		} else {
			_contracts = types.CALL
		}
	}

	var _timeInForce types.TimeInForce
	if !data.TimeInForce.IsValid() {
		_timeInForce = types.GOOD_TIL_CANCELLED
	} else {
		_timeInForce = data.TimeInForce
	}

	upperType := strings.ToUpper(string(data.Type))
	payload := model.DeribitResponse{
		UserId:         userId,
		ClientId:       data.ClientId,
		Underlying:     _underlying,
		ExpirationDate: _expDate,
		StrikePrice:    *strikePrice,
		Type:           types.Type(upperType),
		Side:           data.Side,
		ClOrdID:        data.ClOrdID,
		Price:          data.Price,
		Amount:         data.Amount,
		Contracts:      _contracts,
		TimeInForce:    _timeInForce,
		Label:          data.Label,

		OrderExclusions: data.OrderExclusions,
		TypeInclusions:  data.TypeInclusions,
	}

	out, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	//send to kafka
	producer.KafkaProducer(string(out), "NEW_ORDER")

	return payload, nil
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
