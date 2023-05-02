package repositories

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	engineType "gateway/internal/engine/types"

	deribitModel "gateway/internal/deribit/model"
	tradeType "gateway/internal/repositories/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TradeRepository struct {
	collection *mongo.Collection
}

func NewTradeRepository(db Database) *TradeRepository {
	collection := db.InitCollection("trades")
	return &TradeRepository{collection}
}

func (r TradeRepository) Find(filter interface{}, sort interface{}, offset, limit int64) ([]*engineType.Trade, error) {
	options := options.FindOptions{
		MaxTime: &defaultTimeout,
	}

	if offset >= 0 {
		options.SetSkip(offset)
	}

	if limit >= 0 {
		options.SetLimit(limit)
	}

	if sort != nil {
		options.SetSort(sort)
	}

	if filter == nil {
		filter = bson.M{}
	}

	cursor, err := r.collection.Find(context.Background(), filter, &options)
	if err != nil {
		return nil, err
	}

	defer cursor.Close(context.Background())

	Trades := []*engineType.Trade{}

	err = cursor.All(context.Background(), &Trades)
	if err != nil {
		return nil, err
	}

	return Trades, nil
}

func (r TradeRepository) FindUserTradesByInstrument(
	instrument string,
	sort string,
	count int,
	userId string,
) (result deribitModel.DeribitGetUserTradesByInstrumentsResponse, err error) {
	options := options.AggregateOptions{
		MaxTime: &defaultTimeout,
	}

	_string := instrument
	substring := strings.Split(_string, "-")

	_strikePrice, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		fmt.Println(err)
	}
	_underlying := substring[0]
	_expiryDate := strings.ToUpper(substring[1])

	query := bson.A{
		bson.D{
			{"$lookup",
				bson.D{
					{"from", "orders"},
					{"localField", "takerOrderId"},
					{"foreignField", "_id"},
					{"as", "takerOrder"},
				},
			},
		},
		bson.D{
			{"$lookup",
				bson.D{
					{"from", "orders"},
					{"localField", "makerOrderId"},
					{"foreignField", "_id"},
					{"as", "makerOrder"},
				},
			},
		},
		bson.D{
			{"$match",
				bson.D{
					{"underlying", _underlying},
					{"strikePrice", _strikePrice},
					{"expiryDate", _expiryDate},
					{"$or",
						bson.A{
							bson.D{{"takerId", userId}},
							bson.D{{"makerId", userId}},
						},
					},
				},
			},
		},
		bson.D{
			{"$project",
				bson.D{
					{"amount", "$amount"},
					{"direction", "$side"},
					{"order_id",
						bson.D{
							{"$cond",
								bson.A{
									"$takerId" == userId,
									"$takerOrderId",
									"$makerOrderId",
								},
							},
						},
					},
					{"order_type",
						bson.D{
							{"$cond",
								bson.A{
									"$takerId" == userId,
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$takerOrderId.type",
												0,
											},
										},
									},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$makerOrder.type",
												0,
											},
										},
									},
								},
							},
						},
					},
					{"price", "$price"},
					{"state",
						bson.D{
							{"$cond",
								bson.A{
									"$takerId" == userId,
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$takerOrderId.status",
												0,
											},
										},
									},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$makerOrder.status",
												0,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		bson.D{
			{"$facet",
				bson.D{
					{"trades",
						bson.A{
							bson.D{{"$skip", 0}},
							bson.D{{"$limit", count}},
						},
					},
					{"total",
						bson.A{
							bson.D{{"$count", "count"}},
						},
					},
				},
			},
		},
	}

	var cursor *mongo.Cursor
	cursor, err = r.collection.Aggregate(context.Background(), query, &options)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cursor.Close(context.Background())

	var res tradeType.UserTradesByInstrumentResult
	for cursor.Next(context.TODO()) {
		if err = cursor.Decode(&res); err != nil {
			fmt.Println(err)
			return
		}
	}

	result.Trades = res.Trades
	if len(res.Total) > 0 {
		result.HasMore = res.Total[0].Count > int64(count)
	}

	return result, nil
}
