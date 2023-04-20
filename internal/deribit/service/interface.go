package service

import (
	"context"
	"gateway/internal/deribit/model"
)

type IDeribitService interface {
	DeribitParseBuy(ctx context.Context, userID string, data model.DeribitRequest) (model.DeribitResponse, error)
	DeribitParseSell(ctx context.Context, userID string, data model.DeribitRequest) (model.DeribitResponse, error)
	DeribitParseEdit(ctx context.Context, userID string, data model.DeribitEditRequest) (model.DeribitEditResponse, error)
	DeribitParseCancel(ctx context.Context, userID string, data model.DeribitCancelRequest) (model.DeribitCancelResponse, error)
	DeribitCancelByInstrument(ctx context.Context, userID string, data model.DeribitCancelByInstrumentRequest) (model.DeribitCancelByInstrumentResponse, error)
}
