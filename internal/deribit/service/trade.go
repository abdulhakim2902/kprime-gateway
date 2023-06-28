package service

import (
	"context"
	_deribitModel "gateway/internal/deribit/model"

	"git.devucc.name/dependencies/utilities/types/validation_reason"
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

func (svc *deribitService) GetTradingViewChartData(ctx context.Context, request _deribitModel.GetTradingviewChartDataRequest) (trades _deribitModel.GetTradingviewChartDataResponse, reason *validation_reason.ValidationReason, err error) {
	trades, reason, err = svc.tradeRepo.GetTradingViewChartData(request)

	return
}
