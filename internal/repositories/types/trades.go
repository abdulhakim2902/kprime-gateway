package types

import (
	deribitModel "gateway/internal/deribit/model"
)

type UserTradesByInstrumentResult struct {
	Trades []*deribitModel.DeribitGetUserTradesByInstruments `bson:"trades"`
	Total  []*struct {
		Count int64 `bson:"count"`
	} `bson:"total"`
}

type UserTradesByOderResult struct {
	Trades []*deribitModel.DeribitGetUserTradesByOrderValue `bson:"trades"`
	Total  []*struct {
		Count int64 `bson:"count"`
	} `bson:"total"`
}
