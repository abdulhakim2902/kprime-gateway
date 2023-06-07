package service

import (
	"context"
	"gateway/internal/deribit/model"

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
	DeribitGetLastTradesByInstrument(ctx context.Context, data model.DeribitGetLastTradesByInstrumentRequest) []*model.DeribitGetLastTradesByInstrumentResponse
	GetOrderBook(ctx context.Context, data model.DeribitGetOrderBookRequest) *model.DeribitGetOrderBookResponse
	GetIndexPrice(ctx context.Context, data model.DeribitGetIndexPriceRequest) model.DeribitGetIndexPriceResponse
	GetDeliveryPrices(ctx context.Context, request model.DeliveryPricesRequest) model.DeliveryPricesResponse

	DeribitGetOrderStateByLabel(ctx context.Context, data model.DeribitGetOrderStateByLabelRequest) []*model.DeribitGetOrderStateByLabelResponse
	DeribitGetOrderState(ctx context.Context, userId string, request model.DeribitGetOrderStateRequest) []model.DeribitGetOrderStateResponse
	DeribitGetUserTradesByOrder(ctx context.Context, userId string, InstrumentName string, request model.DeribitGetUserTradesByOrderRequest) []model.DeribitGetUserTradesByOrderResponse
}
