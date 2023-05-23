package repositories

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	_deribitModel "gateway/internal/deribit/model"
	_engineType "gateway/internal/engine/types"
	_tradeType "gateway/internal/repositories/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

func (r TradeRepository) Find(filter interface{}, sort interface{}, offset, limit int64) ([]*_engineType.Trade, error) {
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

	Trades := []*_engineType.Trade{}

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
) (result _deribitModel.DeribitGetUserTradesByInstrumentsResponse, err error) {
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
					{"localField", "taker.orderId"},
					{"foreignField", "_id"},
					{"as", "takerOrder"},
				},
			},
		},
		bson.D{
			{"$lookup",
				bson.D{
					{"from", "orders"},
					{"localField", "maker.orderId"},
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
							bson.D{{"taker.userId", userId}},
							bson.D{{"maker.userId", userId}},
						},
					},
				},
			},
		},
		bson.D{
			{"$project",
				bson.D{
					{"InstrumentName", bson.M{"$concat": bson.A{
						bson.D{
							{"$convert", bson.D{
								{"input", "$underlying"},
								{"to", "string"},
							}}},
						"-",
						bson.D{
							{"$convert", bson.D{
								{"input", "$expiryDate"},
								{"to", "string"},
							}}},
						"-",
						bson.D{
							{"$convert", bson.D{
								{"input", "$strikePrice"},
								{"to", "string"},
							}}},
						"-",
						bson.M{"$substr": bson.A{"$contracts", 0, 1}},
					}}},
					{"amount", "$amount"},
					{"direction", "$side"},
					{"label",
						bson.D{
							{"$cond",
								bson.A{
									"$taker.userId" == userId,
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$takerOrder.label",
												0,
											},
										},
									},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$makerOrder.label",
												0,
											},
										},
									},
								},
							},
						},
					},
					{"order_id",
						bson.D{
							{"$cond",
								bson.A{
									"$taker.userId" == userId,
									"$taker.orderId",
									"$maker.orderId",
								},
							},
						},
					},
					{"order_type",
						bson.D{
							{"$cond",
								bson.A{
									"$taker.userId" == userId,
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$takerOrder.type",
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
					{"timestamp", bson.M{"$toLong": "$createdAt"}},
					{"strikePrice", "$strikePrice"},
					{"tickDirection", "$tickDirection"},
					{"tradeSequence", "$tradeSequence"},
					{"indexPrice", "$indexPrice"},
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

	var res _tradeType.UserTradesByInstrumentResult
	for cursor.Next(context.TODO()) {
		if err = cursor.Decode(&res); err != nil {
			fmt.Println(err)
			return
		}
		for _, trade := range res.Trades {
			trade.Api = true
		}
	}

	result.Trades = res.Trades
	if len(res.Total) > 0 {
		result.HasMore = res.Total[0].Count > int64(count)
	}

	return result, nil
}

func (r TradeRepository) FindUserTradesById(
	instrument string,
	userId []interface{},
	orderId []interface{},
) (result _tradeType.UserTradesByInstrumentResult, err error) {
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
					{"localField", "taker.orderId"},
					{"foreignField", "_id"},
					{"as", "takerOrder"},
				},
			},
		},
		bson.D{
			{"$lookup",
				bson.D{
					{"from", "orders"},
					{"localField", "maker.orderId"},
					{"foreignField", "_id"},
					{"as", "makerOrder"},
				},
			},
		},
		bson.D{
			{"$match",
				bson.D{
					{"_id", bson.M{"$in": orderId}},
					{"underlying", _underlying},
					{"strikePrice", _strikePrice},
					{"expiryDate", _expiryDate},
					{"$or",
						bson.A{
							bson.M{"taker.userId": bson.M{"$in": userId}},
							bson.M{"maker.userId": bson.M{"$in": userId}},
						},
					},
				},
			},
		},
		bson.D{
			{"$project",
				bson.D{
					{"InstrumentName", bson.M{"$concat": bson.A{
						bson.D{
							{"$convert", bson.D{
								{"input", "$underlying"},
								{"to", "string"},
							}}},
						"-",
						bson.D{
							{"$convert", bson.D{
								{"input", "$expiryDate"},
								{"to", "string"},
							}}},
						"-",
						bson.D{
							{"$convert", bson.D{
								{"input", "$strikePrice"},
								{"to", "string"},
							}}},
						"-",
						bson.M{"$substr": bson.A{"$contracts", 0, 1}},
					}}},
					{"amount", "$amount"},
					{"direction", "$side"},
					{"label",
						bson.D{
							{"$cond",
								bson.A{
									bson.M{"$in": bson.A{"$userId", userId}},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$takerOrder.label",
												0,
											},
										},
									},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$makerOrder.label",
												0,
											},
										},
									},
								},
							},
						},
					},
					{"order_id",
						bson.D{
							{"$cond",
								bson.A{
									bson.M{"$in": bson.A{"$userId", userId}},
									"$taker.orderId",
									"$maker.orderId",
								},
							},
						},
					},
					{"order_type",
						bson.D{
							{"$cond",
								bson.A{
									bson.M{"$in": bson.A{"$userId", userId}},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$takerOrder.type",
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
									bson.M{"$in": bson.A{"$userId", userId}},
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
					{"timestamp", bson.M{"$toLong": "$createdAt"}},
					{"strikePrice", "$strikePrice"},
					{"tickDirection", "$tickDirection"},
					{"tradeSequence", "$tradeSequence"},
					{"indexPrice", "$indexPrice"},
				},
			},
		},
		bson.D{
			{"$facet",
				bson.D{
					{"trades",
						bson.A{
							bson.D{{"$skip", 0}},
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

	var res _tradeType.UserTradesByInstrumentResult
	for cursor.Next(context.TODO()) {
		if err = cursor.Decode(&res); err != nil {
			fmt.Println(err)
			return
		}
		for _, trade := range res.Trades {
			trade.Api = true
		}
	}

	result.Trades = res.Trades

	return result, nil
}

func (r TradeRepository) FindTradesByInstrument(
	instrument string,
	orderId []interface{},
) (result _tradeType.UserTradesByInstrumentResult, err error) {
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
					{"localField", "taker.orderId"},
					{"foreignField", "_id"},
					{"as", "takerOrder"},
				},
			},
		},
		bson.D{
			{"$lookup",
				bson.D{
					{"from", "orders"},
					{"localField", "maker.orderId"},
					{"foreignField", "_id"},
					{"as", "makerOrder"},
				},
			},
		},
		bson.D{
			{"$match",
				bson.D{
					{"_id", bson.M{"$in": orderId}},
					{"underlying", _underlying},
					{"strikePrice", _strikePrice},
					{"expiryDate", _expiryDate},
				},
			},
		},
		bson.D{
			{"$project",
				bson.D{
					{"InstrumentName", bson.M{"$concat": bson.A{
						bson.D{
							{"$convert", bson.D{
								{"input", "$underlying"},
								{"to", "string"},
							}}},
						"-",
						bson.D{
							{"$convert", bson.D{
								{"input", "$expiryDate"},
								{"to", "string"},
							}}},
						"-",
						bson.D{
							{"$convert", bson.D{
								{"input", "$strikePrice"},
								{"to", "string"},
							}}},
						"-",
						bson.M{"$substr": bson.A{"$contracts", 0, 1}},
					}}},
					{"amount", "$amount"},
					{"direction", "$side"},
					{"label",
						bson.D{
							{"$arrayElemAt",
								bson.A{
									"$takerOrder.label",
									0,
								},
							},
						},
					},
					{"order_id",
						bson.D{
							{"$ifNull",
								bson.A{
									"$taker.orderId",
									"$taker.orderId",
									"$maker.orderId",
								},
							},
						},
					},
					{"order_type",
						bson.D{
							{"$arrayElemAt",
								bson.A{
									"$takerOrder.type",
									0,
								},
							},
						},
					},
					{"price", "$price"},
					{"state",
						bson.D{
							{"$arrayElemAt",
								bson.A{
									"$takerOrderId.status",
									0,
								},
							},
						},
					},
					{"timestamp", bson.M{"$toLong": "$createdAt"}},
					{"strikePrice", "$strikePrice"},
					{"tickDirection", "$tickDirection"},
					{"tradeSequence", "$tradeSequence"},
					{"indexPrice", "$indexPrice"},
				},
			},
		},
		bson.D{
			{"$facet",
				bson.D{
					{"trades",
						bson.A{
							bson.D{{"$skip", 0}},
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

	var res _tradeType.UserTradesByInstrumentResult
	for cursor.Next(context.TODO()) {
		if err = cursor.Decode(&res); err != nil {
			fmt.Println(err)
			return
		}
		for _, trade := range res.Trades {
			trade.Api = true
		}
	}

	result.Trades = res.Trades

	return result, nil
}

func (r TradeRepository) GetPriceAvg(underlying, expiryDate, contracts string, strikePrice float64) (price float64, err error) {
	options := options.AggregateOptions{
		MaxTime: &defaultTimeout,
	}

	query := bson.A{
		bson.D{
			{"$match",
				bson.D{
					{"underlying", underlying},
					{"strikePrice", strikePrice},
					{"expiryDate", expiryDate},
					{"contracts", contracts},
				},
			},
		},
		bson.D{
			{"$group",
				bson.D{
					{"_id", primitive.Null{}},
					{"price", bson.D{{"$avg", "$price"}}},
				},
			},
		},
		bson.D{
			{"$unset",
				bson.A{
					"_id",
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

	var avgPrice map[string]interface{}
	for cursor.Next(context.TODO()) {
		if err = cursor.Decode(&avgPrice); err != nil {
			fmt.Println(err)
			return
		}
	}

	if val, ok := avgPrice["price"]; ok {
		price = val.(float64)
	}

	return
}
