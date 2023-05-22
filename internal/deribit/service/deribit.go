package service

import (
	"context"
	"encoding/json"
	"errors"
	"gateway/internal/deribit/model"
	"gateway/internal/repositories"
	"gateway/pkg/kafka/producer"
	"gateway/pkg/redis"
	"gateway/pkg/utils"
	"strings"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/models/order"
	"git.devucc.name/dependencies/utilities/types"
	"git.devucc.name/dependencies/utilities/types/validation_reason"
)

type deribitService struct {
	userRepo            *repositories.UserRepository
	tradeRepo           *repositories.TradeRepository
	orderRepo           *repositories.OrderRepository
	rawPriceRepo        *repositories.RawPriceRepository
	settlementPriceRepo *repositories.SettlementPriceRepository

	redis *redis.RedisConnectionPool
}

func NewDeribitService(
	redis *redis.RedisConnectionPool,

	userRepo *repositories.UserRepository,
	tradeRepo *repositories.TradeRepository,
	orderRepo *repositories.OrderRepository,
	rawPriceRepo *repositories.RawPriceRepository,
	settlementPriceRepo *repositories.SettlementPriceRepository,
) IDeribitService {
	return &deribitService{
		userRepo,
		tradeRepo,
		orderRepo,
		rawPriceRepo,
		settlementPriceRepo,
		redis,
	}
}

func (svc deribitService) DeribitRequest(
	ctx context.Context,
	userId string,
	data model.DeribitRequest,
) (*model.DeribitResponse, *validation_reason.ValidationReason, error) {

	instruments, err := utils.ParseInstruments(data.InstrumentName)
	if err != nil {
		reason := validation_reason.INVALID_PARAMS
		return nil, &reason, err
	}

	upperType := strings.ToUpper(string(data.Type))
	userHasOrderType := false

	user, err := svc.userRepo.FindById(context.TODO(), userId)
	if err != nil {
		if strings.Contains(err.Error(), "no documents in result") {
			reason := validation_reason.UNAUTHORIZED
			return nil, &reason, errors.New(reason.String())
		}
		logs.Log.Error().Err(err).Msg("")

		return nil, nil, err
	}

	var typeInclusions []order.TypeInclusions
	for _, orderType := range user.OrderTypes {
		if strings.ToLower(orderType.Name) == strings.ToLower(upperType) {
			userHasOrderType = true
		}

		typeInclusions = append(typeInclusions, order.TypeInclusions{
			Name: orderType.Name,
		})
	}

	if !userHasOrderType {
		reason := validation_reason.ORDER_TYPE_NO_MATCH
		return nil, &reason, errors.New(reason.String())
	}

	var orderExclusions []order.OrderExclusion
	for _, item := range user.OrderExclusions {
		orderExclusions = append(orderExclusions, order.OrderExclusion{
			UserID: item.UserID,
		})
	}

	var _timeInForce types.TimeInForce
	if !data.TimeInForce.IsValid() {
		_timeInForce = types.GOOD_TIL_CANCELLED
	} else {
		_timeInForce = data.TimeInForce
	}

	payload := model.DeribitResponse{
		UserId:         userId,
		ClientId:       data.ClientId,
		Underlying:     instruments.Underlying,
		ExpirationDate: instruments.ExpDate,
		StrikePrice:    instruments.Strike,
		Type:           types.Type(upperType),
		Side:           data.Side,
		ClOrdID:        data.ClOrdID,
		Price:          data.Price,
		Amount:         data.Amount,
		Contracts:      instruments.Contracts,
		TimeInForce:    _timeInForce,
		Label:          data.Label,

		OrderExclusions: orderExclusions,
		TypeInclusions:  typeInclusions,
	}

	out, err := json.Marshal(payload)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil, nil, err
	}
	//send to kafka
	producer.KafkaProducer(string(out), "NEW_ORDER")

	return &payload, nil, nil
}

func (svc deribitService) DeribitParseEdit(ctx context.Context, userId string, data model.DeribitEditRequest) (*model.DeribitEditResponse, error) {

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
		logs.Log.Error().Err(err).Msg("")

		return nil, err
	}
	//send to kafka
	producer.KafkaProducer(string(_edit), "NEW_ORDER")

	return &edit, nil
}

func (svc deribitService) DeribitParseCancel(ctx context.Context, userId string, data model.DeribitCancelRequest) (*model.DeribitCancelResponse, error) {
	cancel := model.DeribitCancelResponse{
		Id:       data.Id,
		UserId:   userId,
		ClientId: "",
		Side:     "CANCEL",
		ClOrdID:  data.ClOrdID,
	}

	_cancel, err := json.Marshal(cancel)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil, err
	}
	//send to kafka
	producer.KafkaProducer(string(_cancel), "NEW_ORDER")

	return &cancel, nil
}

func (svc deribitService) DeribitCancelByInstrument(ctx context.Context, userId string, data model.DeribitCancelByInstrumentRequest) (*model.DeribitCancelByInstrumentResponse, error) {
	instruments, err := utils.ParseInstruments(data.InstrumentName)
	if err != nil {
		return nil, err
	}

	cancel := model.DeribitCancelByInstrumentResponse{
		UserId:         userId,
		ClientId:       "",
		Underlying:     instruments.Underlying,
		ExpirationDate: instruments.ExpDate,
		StrikePrice:    instruments.Strike,
		Contracts:      instruments.Contracts,
		Side:           "CANCEL_ALL_BY_INSTRUMENT",
		ClOrdID:        data.ClOrdID,
	}

	_cancel, err := json.Marshal(cancel)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil, err
	}
	//send to kafka
	producer.KafkaProducer(string(_cancel), "NEW_ORDER")

	return &cancel, nil
}

func (svc deribitService) DeribitParseCancelAll(ctx context.Context, userId string, data model.DeribitCancelAllRequest) (*model.DeribitCancelAllResponse, error) {
	cancel := model.DeribitCancelAllResponse{
		UserId:   userId,
		ClientId: "",
		Side:     "CANCEL_ALL",
		ClOrdID:  data.ClOrdID,
	}

	_cancel, err := json.Marshal(cancel)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil, err
	}
	//send to kafka
	producer.KafkaProducer(string(_cancel), "NEW_ORDER")

	return &cancel, nil
}
