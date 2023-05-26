package service

import (
	"context"
	_deribitModel "gateway/internal/deribit/model"
)

func (svc deribitService) DeribitGetUserTradesByInstrument(
	ctx context.Context,
	userId string,
	request _deribitModel.DeribitGetUserTradesByInstrumentsRequest,
) *_deribitModel.DeribitGetUserTradesByInstrumentsResponse {

	trades, err := svc.tradeRepo.FindUserTradesByInstrument(
		request.InstrumentName,
		request.Sorting,
		request.Count,
		userId,
	)
	if err != nil {
		return nil
	}

	return &trades
}