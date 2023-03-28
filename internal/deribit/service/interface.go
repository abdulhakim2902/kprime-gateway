package service

import (
	"context"
	"net/url"
)

type IDeribitService interface {
	DeribitParseOrder(ctx context.Context, params url.Values) (interface{}, error)
}
