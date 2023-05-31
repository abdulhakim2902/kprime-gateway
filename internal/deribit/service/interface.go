package service

import (
	"context"
	"gateway/internal/deribit/model"
	orderbookType "gateway/internal/orderbook/types"

	"git.devucc.name/dependencies/utilities/types/validation_reason"
)

type IDeribitService interface {
	DeribitRequest(ctx context.Context, userID string, data model.DeribitRequest) (*model.DeribitResponse, *validation_reason.ValidationReason, error)
	DeribitParseEdit(ctx context.Context, userID string, data model.DeribitEditRequest) (*model.DeribitEditResponse, error)
	DeribitParseCancel(ctx context.Context, userID string, data model.DeribitCancelRequest) (*model.DeribitCancelResponse, error)
	DeribitCancelByInstrument(ctx context.Context, userID string, data model.DeribitCancelByInstrumentRequest) (*model.DeribitCancelByInstrumentResponse, error)
	DeribitParseCancelAll(ctx context.Context, userID string, data model.DeribitCancelAllRequest) (*model.DeribitCancelAllResponse, error)

	DeribitGetUserTradesByInstrument(ctx context.Context, userID string, data model.DeribitGetUserTradesByInstrumentsRequest) *model.DeribitGetUserTradesByInstrumentsResponse
	DeribitGetOpenOrdersByInstrument(ctx context.Context, userID string, data model.DeribitGetOpenOrdersByInstrumentRequest) []*model.DeribitGetOpenOrdersByInstrumentResponse
	DeribitGetOrderHistoryByInstrument(ctx context.Context, userID string, data model.DeribitGetOrderHistoryByInstrumentRequest) []*model.DeribitGetOrderHistoryByInstrumentResponse

	DeribitGetInstruments(ctx context.Context, data model.DeribitGetInstrumentsRequest) []*model.DeribitGetInstrumentsResponse
	GetOrderBook(ctx context.Context, data model.DeribitGetOrderBookRequest) *model.DeribitGetOrderBookResponse
	GetIndexPrice(ctx context.Context, data model.DeribitGetIndexPriceRequest) model.DeribitGetIndexPriceResponse

	DeribitGetOrderStateByLabel(ctx context.Context, data model.DeribitGetOrderStateByLabelRequest) []*orderbookType.Order
}
