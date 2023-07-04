package service

import (
	"context"
	"encoding/json"
	"gateway/internal/deribit/model"

	"git.devucc.name/dependencies/utilities/commons/logs"
)

func (svc deribitService) DeribitGetOpenOrdersByInstrument(ctx context.Context, userID string, data model.DeribitGetOpenOrdersByInstrumentRequest) []*model.DeribitGetOpenOrdersByInstrumentResponse {
	trades, err := svc.orderRepo.GetOpenOrdersByInstrument(
		data.InstrumentName,
		data.Type,
		userID,
	)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil
	}

	jsonBytes, err := json.Marshal(trades)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil
	}

	var openOrderData []*model.DeribitGetOpenOrdersByInstrumentResponse
	if err = json.Unmarshal([]byte(jsonBytes), &openOrderData); err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil
	}

	return openOrderData
}
func (svc deribitService) DeribitGetOrderHistoryByInstrument(ctx context.Context, userID string, data model.DeribitGetOrderHistoryByInstrumentRequest) []*model.DeribitGetOrderHistoryByInstrumentResponse {
	trades, err := svc.orderRepo.GetOrderHistoryByInstrument(
		data.InstrumentName,
		data.Count,
		data.Offset,
		data.IncludeOld,
		data.IncludeUnfilled,
		userID,
	)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil
	}

	jsonBytes, err := json.Marshal(trades)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil
	}

	var historyOrderData []*model.DeribitGetOrderHistoryByInstrumentResponse
	err = json.Unmarshal([]byte(jsonBytes), &historyOrderData)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil
	}

	return historyOrderData
}

func (svc deribitService) DeribitGetInstruments(ctx context.Context, data model.DeribitGetInstrumentsRequest) []*model.DeribitGetInstrumentsResponse {
	// key := "INSTRUMENTS-" + data.Currency + "" + strconv.FormatBool(data.Expired)

	// // Get initial data from the redis
	// res, err := svc.redis.GetValue(key)

	// // Handle the initial data
	// if res == "" || err != nil {
	// 	logs.Log.Error().Err(err).Msg("")

	// Get All Orders, and Save it to the redis
	orders, err := svc.orderRepo.GetInstruments(data.UserId, data.Currency, data.Expired)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
	}

	// jsonBytes, err := json.Marshal(orders)
	// if err != nil {
	// 	logs.Log.Error().Err(err).Msg("")
	// }
	// Expire in seconds
	// svc.redis.SetEx(key, string(jsonBytes), 3)

	// res, _ := svc.redis.GetValue(key)
	// }

	// var instrumentData []*model.DeribitGetInstrumentsResponse
	// if err = json.Unmarshal([]byte(res), &instrumentData); err != nil {
	// 	logs.Log.Error().Err(err).Msg("")
	// 	return nil
	// }

	return orders
}

func (svc deribitService) DeribitGetOrderState(ctx context.Context, userId string, request model.DeribitGetOrderStateRequest) []model.DeribitGetOrderStateResponse {
	orders, err := svc.orderRepo.GetOrderState(
		userId,
		request.OrderId,
	)
	if err != nil {
		return nil
	}

	jsonBytes, err := json.Marshal(orders)
	if err != nil {
		return nil
	}

	var orderState []model.DeribitGetOrderStateResponse
	err = json.Unmarshal([]byte(jsonBytes), &orderState)
	if err != nil {
		return nil
	}

	return orderState
}

func (svc deribitService) DeribitGetOrderStateByLabel(ctx context.Context, data model.DeribitGetOrderStateByLabelRequest) []*model.DeribitGetOrderStateByLabelResponse {
	orders, err := svc.orderRepo.GetOrderStateByLabel(ctx, data)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil
	}

	if orders == nil {
		return make([]*model.DeribitGetOrderStateByLabelResponse, 0)
	}

	return orders
}
