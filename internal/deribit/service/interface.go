package service

import (
	"context"
	"gateway/internal/deribit/model"
)

type IDeribitService interface {
	DeribitParseBuy(ctx context.Context, userID string, data model.DeribitRequest) (model.DeribitResponse, error)
	DeribitParseSell(ctx context.Context, userID string, data model.DeribitRequest) (model.DeribitResponse, error)
}
