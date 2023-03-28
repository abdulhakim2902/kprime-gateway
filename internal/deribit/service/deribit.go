package service

import (
	"context"
	"log"
	"net/url"
)

type deribitService struct {
	//
}

func NewDeribitService() IDeribitService {
	return &deribitService{}
}

func (svc deribitService) DeribitParseOrder(ctx context.Context, params url.Values) (interface{}, error) {
	log.Println(params)
	return map[string]interface{}{
		"result": "success",
	}, nil
}
