package repositories

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	_deribitModel "gateway/internal/deribit/model"
	_engineType "gateway/internal/engine/types"
	_orderbookType "gateway/internal/orderbook/types"
	_tradeType "gateway/internal/repositories/types"
	"gateway/pkg/memdb"
	"gateway/pkg/utils"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	Greeks "github.com/Undercurrent-Technologies/kprime-utilities/helper/greeks"
	IV "github.com/Undercurrent-Technologies/kprime-utilities/helper/implied_volatility"
	"github.com/Undercurrent-Technologies/kprime-utilities/models/trade"
	"github.com/Undercurrent-Technologies/kprime-utilities/types"
	"github.com/Undercurrent-Technologies/kprime-utilities/types/validation_reason"
	"github.com/shopspring/decimal"

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

func (r TradeRepository) FilterTradesData(data _deribitModel.DeribitGetLastTradesByInstrumentRequest) []*_engineType.Trade {
	fmt.Println(data.StartSeq, " - type : ", reflect.TypeOf(data.StartSeq))

	// Querry for Instrument Name
	str := data.InstrumentName
	components := strings.Split(str, "-")

	underlying := components[0]
	expiryDate := components[1]
	strikePrice := components[2]
	contracts := components[3]

	switch contracts {
	case "C":
		contracts = "CALL"
	case "P":
		contracts = "PUT"
	}

	strikePriceFloat, _ := strconv.ParseFloat(strikePrice, 64)

	filter := bson.M{
		"underlying":  underlying,
		"expiryDate":  expiryDate,
		"strikePrice": strikePriceFloat,
		"contracts":   contracts,
	}

	// Querry for Sequence
	if data.StartSeq != 0 && data.EndSeq != 0 {
		filter["tradeSequence"] = bson.M{
			"$gte": data.StartSeq,
			"$lte": data.EndSeq,
		}
	} else {
		if data.StartSeq != 0 {
			filter["tradeSequence"] = bson.M{"$gte": data.StartSeq}
		}
		if data.EndSeq != 0 {
			filter["tradeSequence"] = bson.M{"$lte": data.EndSeq}
		}
	}

	// Querry for Time Stamp
	if !data.StartTimestamp.IsZero() && !data.EndTimestamp.IsZero() {
		filter["createdAt"] = bson.M{
			"$gte": data.StartTimestamp,
			"$lte": data.EndTimestamp,
		}
	} else {
		if !data.StartTimestamp.IsZero() {
			filter["createdAt"] = bson.M{"$gte": data.StartTimestamp}
		}
		if !data.EndTimestamp.IsZero() {
			filter["createdAt"] = bson.M{"$lte": data.EndTimestamp}
		}
	}

	// Querry for Instrument End Time Stamp

	// Querry for Count
	limit := int64(data.Count)
	if limit == 0 {
		limit = 3
	}

	findOptions := options.Find()
	findOptions.SetLimit(limit)

	sortOrder := -1 // Default sort order is descending

	switch data.Sorting {
	case "asc":
		sortOrder = 1
	case "desc":
		sortOrder = -1
	}

	if data.Sorting == "" {
		// Set your default sort order here
		sortOrder = -1
	}

	findOptions.SetSort(bson.M{"createdAt": sortOrder})

	cursor, err := r.collection.Find(context.Background(), filter, findOptions)
	if err != nil {
		return nil
	}

	defer cursor.Close(context.Background())

	Trades := []*_engineType.Trade{}

	err = cursor.All(context.Background(), &Trades)
	if err != nil {
		return nil
	}

	return Trades
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
	instrumentName string,
	sort string,
	count int,
	userId string,
) (result _deribitModel.DeribitGetUserTradesByInstrumentsResponse, err error) {
	options := options.AggregateOptions{
		MaxTime: &defaultTimeout,
	}

	sortOrder := -1 // Default sort order is descending

	switch sort {
	case "asc":
		sortOrder = 1
	case "desc":
		sortOrder = -1
	}

	instrument, _ := utils.ParseInstruments(instrumentName, false)
	var uId primitive.ObjectID
	uId, err = primitive.ObjectIDFromHex(userId)
	if err != nil {
		return
	}

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
					{"underlying", instrument.Underlying},
					{"strikePrice", instrument.Strike},
					{"expiryDate", instrument.ExpDate},
					{"$or",
						bson.A{
							bson.D{{"maker.userId", uId}},
							bson.D{{"taker.userId", uId}},
						},
					},
				},
			},
		},
		bson.D{
			{"$project",
				bson.D{
					{"InstrumentName",
						bson.D{
							{"$concat",
								bson.A{
									bson.D{{"$convert", bson.D{{"input", "$underlying"}, {"to", "string"}}}},
									"-",
									bson.D{{"$convert", bson.D{{"input", "$expiryDate"}, {"to", "string"}}}},
									"-",
									bson.D{{"$convert", bson.D{{"input", "$strikePrice"}, {"to", "string"}}}},
									"-",
									bson.D{{"$substr", bson.A{"$contracts", 0, 1}}},
								},
							},
						},
					},
					{"amount", bson.D{{"$convert", bson.D{{"input", "$amount"}, {"to", "double"}}}}},
					{"direction", "$side"},
					{"label",
						bson.D{
							{"$cond",
								bson.D{
									{"if", bson.D{{"$eq", bson.A{"$taker.userId", uId}}}},
									{"then", bson.D{{"$arrayElemAt", bson.A{"$takerOrder.label", 0}}}},
									{"else", bson.D{{"$arrayElemAt", bson.A{"$makerOrder.label", 0}}}},
								},
							},
						},
					},
					{"order_id",
						bson.D{
							{"$cond",
								bson.D{
									{"if", bson.D{{"$eq", bson.A{"$taker.userId", uId}}}},
									{"then", "$taker.orderId"},
									{"else", "$maker.orderId"},
								},
							},
						},
					},
					{"order_type",
						bson.D{
							{"$cond",
								bson.D{
									{"if", bson.D{{"$eq", bson.A{"$taker.userId", uId}}}},
									{"then", bson.D{{"$arrayElemAt", bson.A{"$takerOrder.type", 0}}}},
									{"else", bson.D{{"$arrayElemAt", bson.A{"$makerOrder.type", 0}}}},
								},
							},
						},
					},
					{"price", "$price"},
					{"state",
						bson.D{
							{"$cond",
								bson.D{
									{"if", bson.D{{"$eq", bson.A{"$takerId", uId}}}},
									{"then", bson.D{{"$arrayElemAt", bson.A{"$takerOrderId.status", 0}}}},
									{"else", bson.D{{"$arrayElemAt", bson.A{"$makerOrder.status", 0}}}},
								},
							},
						},
					},
					{"timestamp", bson.D{{"$toLong", "$createdAt"}}},
					{"strikePrice", "$strikePrice"},
					{"tickDirection", "$tickDirection"},
					{"tradeSequence", "$tradeSequence"},
					{"indexPrice", "$indexPrice"},
					{"markPrice", bson.M{"$toDouble": "$markPrice"}},
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
							bson.D{
								{"$sort", bson.D{
									{"timestamp", sortOrder},
								}},
							},
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
			trade.UnderlyingPrice = trade.IndexPrice
			trade.UnderlyingIndex = "index_price"
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
					{"amount", bson.D{
						{"$convert", bson.D{
							{"input", "$amount"},
							{"to", "double"},
						}},
					}},
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
			trade.UnderlyingPrice = trade.IndexPrice
			trade.UnderlyingIndex = "index_price"
		}
	}

	result.Trades = res.Trades

	return result, nil
}

func (r TradeRepository) FindTradesEachUser(
	instrument string,
	userId []interface{},
	tradeId []interface{},
	isTaker bool,
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

	var matchUser primitive.E
	var lookup primitive.D
	if isTaker {
		matchUser = bson.E{Key: "taker.userId", Value: bson.M{"$in": userId}}
		lookup = bson.D{
			{"$lookup",
				bson.D{
					{"from", "orders"},
					{"localField", "taker.orderId"},
					{"foreignField", "_id"},
					{"as", "order"},
				},
			},
		}
	} else {
		matchUser = bson.E{Key: "maker.userId", Value: bson.M{"$in": userId}}
		lookup = bson.D{
			{"$lookup",
				bson.D{
					{"from", "orders"},
					{"localField", "maker.orderId"},
					{"foreignField", "_id"},
					{"as", "order"},
				},
			},
		}
	}

	query := bson.A{
		lookup,
		bson.D{
			{"$match",
				bson.D{
					{"_id", bson.M{"$in": tradeId}},
					{"underlying", _underlying},
					{"strikePrice", _strikePrice},
					{"expiryDate", _expiryDate},
					matchUser,
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
					{"amount", bson.D{
						{"$convert", bson.D{
							{"input", "$amount"},
							{"to", "double"},
						}},
					}},
					{"direction", "$side"},
					{"label",
						bson.D{
							{"$arrayElemAt",
								bson.A{
									"$order.label",
									0,
								},
							},
						},
					},
					{"order_id",
						bson.D{
							{"$arrayElemAt",
								bson.A{
									"$order._id",
									0,
								},
							},
						},
					},
					{"order_type",
						bson.D{
							{"$arrayElemAt",
								bson.A{
									"$order.type",
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
									"$order.status",
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
					{"contracts", "$contracts"},
					{"expiryDate", "$expiryDate"},
					{"markPrice", bson.M{"$toDouble": "$markPrice"}},
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
			trade.UnderlyingPrice = trade.IndexPrice
			trade.UnderlyingIndex = "index_price"

			if trade.MarkPrice != 0 {
				var contracts string
				switch trade.Contracts {
				case "CALL":
					contracts = "C"
				case "PUT":
					contracts = "P"
				}
				markIv := r.GetMarkIv(float64(trade.MarkPrice), contracts, trade.ExpirationDate, trade.StrikePrice, trade.IndexPrice)
				trade.MarkIV = markIv
			}
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
					{"amount", bson.D{
						{"$convert", bson.D{
							{"input", "$amount"},
							{"to", "double"},
						}},
					}},
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
									"$takerOrder.status",
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
					{"contracts", "$contracts"},
					{"expiryDate", "$expiryDate"},
					{"markPrice", bson.M{"$toDouble": "$markPrice"}},
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
			trade.UnderlyingPrice = trade.IndexPrice
			trade.UnderlyingIndex = "index_price"

			if trade.MarkPrice != 0 {
				var contracts string
				switch trade.Contracts {
				case "CALL":
					contracts = "C"
				case "PUT":
					contracts = "P"
				}
				markIv := r.GetMarkIv(float64(trade.MarkPrice), contracts, trade.ExpirationDate, trade.StrikePrice, trade.IndexPrice)
				trade.MarkIV = markIv
			}
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

func (r TradeRepository) GetLastTrades(o _orderbookType.GetOrderBook) []*_engineType.Trade {
	tradesQuery := bson.M{
		"underlying":  o.Underlying,
		"strikePrice": o.StrikePrice,
		"expiryDate":  o.ExpiryDate,
	}

	if o.UserRole == types.CLIENT.String() {
		tradesQuery["maker.userId"] = bson.M{"$nin": o.UserOrderExclusions}
		tradesQuery["taker.userId"] = bson.M{"$nin": o.UserOrderExclusions}
	}

	tradesSort := bson.M{
		"createdAt": 1,
	}

	trades, err := r.Find(tradesQuery, tradesSort, 0, -1)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		return nil
	}

	return trades
}

func (r TradeRepository) GetHighLowTrades(o _orderbookType.GetOrderBook, t int) []*_engineType.Trade {
	currentTime := time.Now()
	oneDayAgo := currentTime.AddDate(0, 0, -1)

	tradesQuery := bson.M{
		"underlying":  o.Underlying,
		"strikePrice": o.StrikePrice,
		"expiryDate":  o.ExpiryDate,
		"createdAt": bson.M{
			"$gte": oneDayAgo,
		},
	}

	if o.UserRole == types.CLIENT.String() {
		tradesQuery["maker.userId"] = bson.M{"$nin": o.UserOrderExclusions}
		tradesQuery["taker.userId"] = bson.M{"$nin": o.UserOrderExclusions}
	}

	tradesSort := bson.M{
		"price": t,
	}

	trades, err := r.Find(tradesQuery, tradesSort, 0, -1)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		return nil
	}

	return trades
}

func (r TradeRepository) Get24HoursTrades(o _orderbookType.GetOrderBook) []*_engineType.Trade {
	currentTime := time.Now()
	oneDayAgo := currentTime.AddDate(0, 0, -1)

	tradesQuery := bson.M{
		"underlying":  o.Underlying,
		"strikePrice": o.StrikePrice,
		"expiryDate":  o.ExpiryDate,
		"createdAt": bson.M{
			"$gte": oneDayAgo,
		},
	}

	if o.UserRole == types.CLIENT.String() {
		tradesQuery["maker.userId"] = bson.M{"$nin": o.UserOrderExclusions}
		tradesQuery["taker.userId"] = bson.M{"$nin": o.UserOrderExclusions}
	}

	tradesSort := bson.M{
		"createdAt": 1,
	}

	trades, err := r.Find(tradesQuery, tradesSort, 0, -1)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		return nil
	}

	return trades
}

func (r TradeRepository) GetMarkIv(markPrice float64, contracts string, expiredDate string, strikePrice float64, indexPrice float64) float64 {
	currentDate := time.Now().Format("2006-01-02 15:04:05")
	layoutExpired := "02Jan06"
	layoutCurrent := "2006-01-02 15:04:05"
	expired, _ := time.Parse(layoutExpired, expiredDate)
	current, _ := time.Parse(layoutCurrent, currentDate)
	calculate := float64(expired.Day()) - float64(current.Day())
	dateValue := float64(calculate / 365)

	optionPrice := ""
	if string(contracts) == "C" {
		optionPrice = "call"
	} else {
		optionPrice = "put"
	}

	markIv := r.GetImpliedVolatility(float64(markPrice), optionPrice, float64(indexPrice), float64(strikePrice), float64(dateValue))
	return markIv
}

func (r TradeRepository) GetImpliedVolatility(marketPrice float64, optionPrice string, underlying float64, strikePrice float64, timeeXP float64) float64 {
	expectedCost := marketPrice // The market price of the option
	s := underlying             // Current price of the underlying
	k := strikePrice            // Strike price
	t := timeeXP                // Time to expiration in years
	ar := 0.05                  // Annual risk-free interest rate as a decimal
	callPut := optionPrice      // Type of option priced - "call" or "put"
	estimate := 0.1             // An initial estimate of implied volatility

	impliedData := IV.ImpliedVolatility(expectedCost, s, k, t, ar, callPut, estimate)

	return impliedData
}

func (r TradeRepository) GetGreeks(types string, impliedVolatily float64, optionPrice string, underlying float64, strikePrice float64, timeeXP float64) float64 {
	var delta float64

	if types == "delta" {
		s := underlying        // underlying price
		k := strikePrice       // strike price
		t := timeeXP           // time to maturity
		v := impliedVolatily   // volatility
		ar := 0.0015           // risk-free interest rate
		callPut := optionPrice // call or put

		delta = Greeks.GetDelta(s, k, t, v, ar, callPut)
	} else if types == "vega" {
		s := underlying      // underlying price
		k := strikePrice     // strike price
		t := timeeXP         // time to maturity
		v := impliedVolatily // volatility
		r := 0.0015          // risk-free interest rate

		delta = Greeks.GetVega(s, k, t, v, r)
	} else if types == "gamma" {
		s := underlying      // underlying price
		k := strikePrice     // strike price
		t := timeeXP         // time to maturity
		v := impliedVolatily // volatility
		r := 0.0015          // risk-free interest rate

		delta = Greeks.GetGamma(s, k, t, v, r)
	} else if types == "tetha" {
		s := underlying        // underlying price
		k := strikePrice       // strike price
		t := timeeXP           // time to maturity
		v := impliedVolatily   // volatility
		r := 0.0015            // risk-free interest rate
		callPut := optionPrice // Type of option priced - "call" or "put"
		scale := 365.0         // You can set the scale to a value like 252 (trading days per year), by default is 365

		delta = Greeks.GetTheta(s, k, t, v, r, callPut, scale)
	} else {
		s := underlying        // underlying price
		k := strikePrice       // strike price
		t := timeeXP           // time to maturity
		v := impliedVolatily   // volatility
		r := 0.0015            // risk-free interest rate
		callPut := optionPrice // Type of option priced - "call" or "put"
		scale := 100.0         // You can set the scale to a value like 10000 (rho per 0.01%, or 1BP, change in the risk-free interest rate), by default is 100

		delta = Greeks.GetRho(s, k, t, v, r, callPut, scale)
	}

	return delta
}

func (r TradeRepository) FilterUserTradesByOrder(userId, orderId, sort string) (result _deribitModel.DeribitGetUserTradesByOrderResponse, err error) {
	options := options.AggregateOptions{
		MaxTime: &defaultTimeout,
	}

	sortOrder := -1 // Default sort order is descending

	switch sort {
	case "asc":
		sortOrder = 1
	case "desc":
		sortOrder = -1
	}

	oId, _ := primitive.ObjectIDFromHex(orderId)
	uId, _ := primitive.ObjectIDFromHex(userId)

	lookupTakerOrderStage := bson.D{
		{"$lookup",
			bson.D{
				{"from", "orders"},
				{"localField", "taker.orderId"},
				{"foreignField", "_id"},
				{"as", "takerOrder"},
			},
		},
	}

	lookupMakerOrderStage := bson.D{
		{"$lookup",
			bson.D{
				{"from", "orders"},
				{"localField", "maker.orderId"},
				{"foreignField", "_id"},
				{"as", "makerOrder"},
			},
		},
	}

	matchStage := bson.D{
		{"$match",
			bson.D{
				{"$or",
					bson.A{
						bson.D{{"taker.userId", uId}, {"taker.orderId", oId}},
						bson.D{{"maker.userId", uId}, {"maker.orderId", oId}},
					},
				},
			},
		},
	}

	projectStage := bson.D{
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
				{"amount", bson.D{
					{"$convert", bson.D{
						{"input", "$amount"},
						{"to", "double"},
					}},
				}},
				{"direction", "$side"},
				{"label",
					bson.D{
						{"$cond",
							bson.A{
								bson.D{{"$eq", bson.A{"$taker.orderId", oId}}},
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
								bson.D{{"$eq", bson.A{"$taker.orderId", oId}}},
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
								bson.D{{"$eq", bson.A{"$taker.orderId", oId}}},
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
								bson.D{{"$eq", bson.A{"$taker.orderId", oId}}},
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
				{"markPrice", bson.M{"$toDouble": "$markPrice"}},
			},
		},
	}

	groupStage := bson.D{
		{"$group", bson.D{
			{"_id", nil},
			{"trades", bson.D{{"$push", "$$ROOT"}}},
		}},
	}

	sortStage := bson.D{{"$sort", bson.D{{"createdAt", sortOrder}}}}

	pipeline := mongo.Pipeline{
		lookupMakerOrderStage,
		lookupTakerOrderStage,
		matchStage,
		sortStage,
		projectStage,
		groupStage,
	}

	var cursor *mongo.Cursor
	cursor, err = r.collection.Aggregate(context.Background(), pipeline, &options)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cursor.Close(context.Background())

	var res _tradeType.UserTradesByOderResult
	for cursor.Next(context.TODO()) {
		if err = cursor.Decode(&res); err != nil {
			fmt.Println(err)
			return
		}
		for _, trade := range res.Trades {
			trade.Api = true
			trade.UnderlyingPrice = trade.IndexPrice
			trade.UnderlyingIndex = "index_price"
		}
	}

	result.Trades = res.Trades

	return result, nil
}

func (r TradeRepository) GetTradingViewChartData(req _deribitModel.GetTradingviewChartDataRequest) (res _deribitModel.GetTradingviewChartDataResponse, reason *validation_reason.ValidationReason, err error) {
	user, vr, er := memdb.MDBFindUserById(req.UserId)
	if er != nil {
		logs.Log.Error().Err(err).Msg("")
		err = er
		reason = &vr

		return
	}

	options := options.AggregateOptions{
		MaxTime: &defaultTimeout,
	}

	var instrument *utils.Instruments
	instrument, err = utils.ParseInstruments(req.InstrumentName, false)
	if err != nil {
		vr := validation_reason.INVALID_PARAMS
		reason = &vr

		logs.Log.Error().Err(err).Msg("")
		return
	}

	excludeUserId := []string{}
	if user.Role == types.CLIENT {
		for _, userCast := range user.OrderExclusions {
			excludeUserId = append(excludeUserId, userCast.UserID)
		}
	}

	match := bson.D{
		{"underlying", instrument.Underlying},
		{"strikePrice", instrument.Strike},
		{"expiryDate", instrument.ExpDate},
		{"contracts", instrument.Contracts},
		{"createdAt",
			bson.D{
				{"$gt", time.UnixMilli(req.StartTimestamp)},
				{"$lt", time.UnixMilli(req.EndTimestamp)},
			},
		},
		{"maker.userId", bson.D{{"$nin", excludeUserId}}},
		{"taker.userId", bson.D{{"$nin", excludeUserId}}},
	}

	matchStage := bson.D{{"$match", match}}

	sortStage := bson.D{
		{"$sort", bson.D{
			{"createdAt", -1},
		}},
	}

	pipeline := mongo.Pipeline{matchStage, sortStage}

	var cursor *mongo.Cursor
	cursor, err = r.collection.Aggregate(context.Background(), pipeline, &options)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		return
	}
	defer cursor.Close(context.Background())

	var trades []*trade.Trade
	if err = cursor.All(context.TODO(), &trades); err != nil {
		logs.Log.Error().Err(err).Msg("")
		return
	}

	type resolution struct {
		Start  time.Time
		End    time.Time
		Trades []trade.Trade
	}

	// Resolution
	// (1, 3, 5, 10, 15, 30, 60, 120, 180, 360, 720, 1D)
	resolutionMap := map[string]uint64{
		"1":   1,
		"3":   3,
		"5":   5,
		"10":  10,
		"15":  15,
		"30":  30,
		"60":  60,
		"120": 120,
		"180": 180,
		"360": 360,
		"720": 720,
		"1D":  1440,
	}

	resolutionRange, ok := resolutionMap[req.Resolution]
	if !ok {
		vr := validation_reason.INVALID_PARAMS
		reason = &vr
		err = errors.New("unsupported resolution")
		return
	}

	start := time.UnixMilli(req.StartTimestamp)
	end := time.UnixMilli(req.EndTimestamp)

	if end.Before(start) || start.Equal(end) {
		vr := validation_reason.INVALID_PARAMS
		reason = &vr
		err = errors.New("end timestamp must greater than start timestamp")
		return
	}

	resolutions := []resolution{}
	endTs := end
	for {
		// If endTs Mapping is after request end timestamp
		// stop mapped the resolution
		if endTs.After(end) || (endTs.Equal(end) && len(resolutions) == 1) {
			break
		}

		endTs = start.Add(time.Duration(resolutionRange) * time.Minute)

		// Add resolution
		resolutions = append(resolutions, resolution{
			start,
			endTs,
			[]trade.Trade{},
		})

		// Set endTs as start for the next resolution mapping
		start = endTs
	}

	res = _deribitModel.GetTradingviewChartDataResponse{
		Close:  []float64{},
		Cost:   []float64{},
		High:   []float64{},
		Low:    []float64{},
		Open:   []float64{},
		Ticks:  []int64{},
		Volume: []float64{},
		Status: "no_data",
	}

	if len(trades) == 0 {
		return
	}

	res.Status = "ok"

	for key, reso := range resolutions {
		for _, trade := range trades {
			created := trade.CreatedAt
			if created.After(reso.Start) && created.Before(reso.End) {
				t := resolutions[key].Trades
				t = append(t, *trade)

				sort.Slice(t, func(i, j int) bool {
					return t[i].CreatedAt.Before(t[j].CreatedAt)
				})

				resolutions[key] = resolution{
					Start:  reso.Start,
					End:    reso.End,
					Trades: t,
				}
			}
		}
	}

	for _, reso := range resolutions {
		res.Ticks = append(res.Ticks, reso.End.UnixMilli())

		if len(reso.Trades) == 0 {
			// Get prev trades open
			open := 0.0
			if len(res.Open) > 0 {
				open = res.Open[len(res.Open)-1]
			}
			res.Open = append(res.Open, open)

			// Get prev trades close
			close := 0.0
			if len(res.Close) > 0 {
				close = res.Close[len(res.Close)-1]
			}
			res.Close = append(res.Close, close)

			// Get prev trades low
			low := 0.0
			if len(res.Low) > 0 {
				low = res.Low[len(res.Low)-1]
			}
			res.Low = append(res.Low, low)

			// Get prev trades high
			high := 0.0
			if len(res.High) > 0 {
				high = res.High[len(res.High)-1]
			}
			res.High = append(res.High, high)

			res.Cost = append(res.Cost, 0.0)
			res.Volume = append(res.Volume, 0.0)

			continue
		}

		open := reso.Trades[0]
		res.Open = append(res.Open, open.Price)

		close := reso.Trades[len(reso.Trades)-1]
		res.Close = append(res.Close, close.Price)

		sortedByPrices := reso.Trades

		sort.Slice(sortedByPrices, func(i, j int) bool {
			return sortedByPrices[i].Price < sortedByPrices[j].Price
		})

		low := sortedByPrices[0]
		res.Low = append(res.Low, low.Price)

		high := sortedByPrices[len(sortedByPrices)-1]
		res.High = append(res.High, high.Price)

		var cost, volume float64
		for _, trade := range reso.Trades {
			amount, err := decimal.NewFromString(trade.Amount)
			if err != nil {
				logs.Log.Error().Err(err).Msg("cannot parsed amount")
				continue
			}

			cost += amount.Mul(decimal.NewFromFloat(trade.Price)).InexactFloat64()
			volume += amount.InexactFloat64()
		}

		res.Cost = append(res.Cost, cost)
		res.Volume = append(res.Volume, volume)
	}

	return
}
