package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gateway/internal/deribit/model"
	"gateway/internal/repositories"
	"gateway/pkg/collector"
	"gateway/pkg/kafka/producer"
	"gateway/pkg/memdb"
	"gateway/pkg/redis"
	"gateway/pkg/utils"
	"log"
	"strings"
	"time"

	userSchema "gateway/schema"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"github.com/Undercurrent-Technologies/kprime-utilities/types"
	"github.com/Undercurrent-Technologies/kprime-utilities/types/validation_reason"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type deribitService struct {
	tradeRepo           *repositories.TradeRepository
	orderRepo           *repositories.OrderRepository
	rawPriceRepo        *repositories.RawPriceRepository
	settlementPriceRepo *repositories.SettlementPriceRepository

	redis *redis.RedisConnectionPool
}

func NewDeribitService(
	redis *redis.RedisConnectionPool,

	tradeRepo *repositories.TradeRepository,
	orderRepo *repositories.OrderRepository,
	rawPriceRepo *repositories.RawPriceRepository,
	settlementPriceRepo *repositories.SettlementPriceRepository,
) IDeribitService {
	return &deribitService{
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
	fmt.Println("debug DeribitRequest", userId)
	instruments, err := utils.ParseInstruments(data.InstrumentName)
	if err != nil {
		reason := validation_reason.INVALID_PARAMS
		return nil, &reason, err
	}

	user, err := memdb.Schemas.User.FindOne("id", userId)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil, nil, err
	}

	if user == nil {
		reason := validation_reason.UNAUTHORIZED
		return nil, &reason, errors.New(reason.String())
	}

	userCast, ok := user.(userSchema.User)
	if !ok {
		reason := validation_reason.UNAUTHORIZED
		return nil, &reason, errors.New(reason.String())
	}
	upperType := strings.ToUpper(string(data.Type))
	userHasOrderType := false

	for _, orderType := range userCast.TypeInclusions {
		if strings.EqualFold(strings.ToLower(orderType.Name), strings.ToLower(upperType)) {
			userHasOrderType = true
			break
		}
	}

	if !userHasOrderType {
		reason := validation_reason.ORDER_TYPE_NO_MATCH
		return nil, &reason, errors.New(reason.String())
	}

	var _timeInForce types.TimeInForce
	if !data.TimeInForce.IsValid() {
		_timeInForce = types.GOOD_TIL_CANCELLED
	} else {
		_timeInForce = data.TimeInForce
	}

	payload := model.DeribitResponse{
		ID:              data.ID,
		UserId:          userId,
		ClientId:        data.ClientId,
		Underlying:      instruments.Underlying,
		ExpirationDate:  instruments.ExpDate,
		StrikePrice:     instruments.Strike,
		Type:            types.Type(strings.ToLower(string(data.Type))),
		Side:            data.Side,
		ClOrdID:         data.ClOrdID,
		Price:           data.Price,
		Amount:          data.Amount,
		Contracts:       instruments.Contracts,
		TimeInForce:     _timeInForce,
		Label:           data.Label,
		MaxShow:         data.MaxShow,
		ReduceOnly:      data.ReduceOnly,
		PostOnly:        data.PostOnly,
		UserRole:        userCast.Role,
		OrderExclusions: userCast.OrderExclusions,
		TypeInclusions:  userCast.TypeInclusions,
	}
	if data.EnableCancel {
		payload.ConnectionId = data.ConnectionId
	}
	out, err := json.Marshal(payload)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil, nil, err
	}

	// collector
	collector.StartKafkaDuration(payload.UserId, payload.ClOrdID)

	//send to kafka
	producer.KafkaProducer(string(out), "NEW_ORDER")

	return &payload, nil, nil
}

func (svc deribitService) DeribitParseEdit(ctx context.Context, userId string, data model.DeribitEditRequest) (*model.DeribitEditResponse, *validation_reason.ValidationReason, error) {

	user, err := memdb.Schemas.User.FindOne("id", userId)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		reason := validation_reason.OTHER

		return nil, &reason, err
	}

	if user == nil {
		reason := validation_reason.UNAUTHORIZED
		return nil, &reason, errors.New(reason.String())
	}

	userCast, ok := user.(userSchema.User)
	if !ok {
		reason := validation_reason.UNAUTHORIZED
		return nil, &reason, errors.New(reason.String())
	}

	edit := model.DeribitEditResponse{
		Id:              data.Id,
		UserId:          userId,
		ClientId:        "",
		Side:            string(types.EDIT),
		ClOrdID:         data.ClOrdID,
		Price:           data.Price,
		Amount:          data.Amount,
		OrderExclusions: userCast.OrderExclusions,
		TypeInclusions:  userCast.TypeInclusions,
	}

	_edit, err := json.Marshal(edit)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		reason := validation_reason.OTHER
		return nil, &reason, errors.New(reason.String())
	}
	//send to kafka
	producer.KafkaProducer(string(_edit), "NEW_ORDER")

	return &edit, nil, err
}

func (svc deribitService) DeribitParseCancel(ctx context.Context, userId string, data model.DeribitCancelRequest) (*model.DeribitCancelResponse, error) {
	cancel := model.DeribitCancelResponse{
		Id:       data.Id,
		UserId:   userId,
		ClientId: "",
		Side:     string(types.CANCEL),
		ClOrdID:  data.ClOrdID,
	}

	_cancel, err := json.Marshal(cancel)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil, err
	}

	// collector
	collector.StartKafkaDuration(cancel.UserId, cancel.ClOrdID)

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
		Side:           string(types.CANCEL_ALL_BY_INSTRUMENT),
		ClOrdID:        data.ClOrdID,
	}

	_cancel, err := json.Marshal(cancel)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil, err
	}

	// collector
	collector.StartKafkaDuration(cancel.UserId, cancel.ClOrdID)

	//send to kafka
	producer.KafkaProducer(string(_cancel), "NEW_ORDER")

	return &cancel, nil
}

func (svc deribitService) DeribitParseCancelAll(ctx context.Context, userId string, data model.DeribitCancelAllRequest) (*model.DeribitCancelAllResponse, error) {
	cancel := model.DeribitCancelAllResponse{
		UserId:   userId,
		ClientId: "",
		Side:     string(types.CANCEL_ALL),
		ClOrdID:  data.ClOrdID,
	}

	_cancel, err := json.Marshal(cancel)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil, err
	}

	// collector
	collector.StartKafkaDuration(cancel.UserId, cancel.ClOrdID)

	//send to kafka
	producer.KafkaProducer(string(_cancel), "NEW_ORDER")

	return &cancel, nil
}

func (svc *deribitService) DeribitGetLastTradesByInstrument(ctx context.Context, data model.DeribitGetLastTradesByInstrumentRequest) model.DeribitGetLastTradesByInstrumentResponse {
	_filteredGets := svc.tradeRepo.FilterTradesData(data)

	bsonResponse := _filteredGets

	_getLastTradesByInstrument := []model.DeribitGetLastTradesByInstrumentValue{}

	for _, doc := range bsonResponse {
		bsonData, err := bson.Marshal(doc)
		if err != nil {
			log.Println("Error marshaling BSON to JSON:", err)
			continue
		}

		var jsonDoc map[string]interface{}
		err = bson.Unmarshal(bsonData, &jsonDoc)
		if err != nil {
			log.Println("Error unmarshaling BSON to JSON:", err)
			continue
		}

		underlying := jsonDoc["underlying"].(string)
		expiryDate := jsonDoc["expiryDate"].(string)
		strikePrice := jsonDoc["strikePrice"].(float64)
		contracts := jsonDoc["contracts"].(string)

		switch contracts {
		case "CALL":
			contracts = "C"
		case "PUT":
			contracts = "P"
		}

		tradeObjectId := jsonDoc["_id"].(primitive.ObjectID)
		conversion, _ := utils.ConvertToFloat(jsonDoc["amount"].(string))
		resultData := model.DeribitGetLastTradesByInstrumentValue{
			Amount:         conversion,
			Direction:      jsonDoc["side"].(string),
			InstrumentName: fmt.Sprintf("%s-%s-%d-%s", underlying, expiryDate, int64(strikePrice), contracts),
			Price:          jsonDoc["price"].(float64),
			Timestamp:      time.Now().UnixNano() / int64(time.Millisecond),
			TradeId:        tradeObjectId.Hex(),
			Api:            true,
			IndexPrice:     jsonDoc["indexPrice"].(float64),
			TickDirection:  jsonDoc["tickDirection"].(int32),
			TradeSeq:       jsonDoc["tradeSequence"].(int32),
			CreatedAt:      jsonDoc["createdAt"].(primitive.DateTime).Time(),
		}

		_getLastTradesByInstrument = append(_getLastTradesByInstrument, resultData)
	}

	results := model.DeribitGetLastTradesByInstrumentResponse{
		Trades: _getLastTradesByInstrument,
	}
	return results
}

func (svc *deribitService) DeribitGetUserTradesByOrder(ctx context.Context, userId string, data model.DeribitGetUserTradesByOrderRequest) *model.DeribitGetUserTradesByOrderResponse {
	trades, err := svc.tradeRepo.FilterUserTradesByOrder(
		userId,
		data.OrderId,
	)
	if err != nil {
		return nil
	}

	jsonBytes, err := json.Marshal(trades)
	if err != nil {
		fmt.Println(err)

		return nil
	}

	var out *model.DeribitGetUserTradesByOrderResponse
	if err = json.Unmarshal([]byte(jsonBytes), &out); err != nil {
		fmt.Println(err)

		return nil
	}

	return out
}
