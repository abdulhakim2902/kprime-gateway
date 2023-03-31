package service

import (
	"context"
	"gateway/internal/deribit/model"
)

type IDeribitService interface {
	DeribitParseBuy(ctx context.Context, data model.DeribitRequest) (model.DeribitResponse, error)
	DeribitParseSell(ctx context.Context, data model.DeribitRequest) (model.DeribitResponse, error)
}
