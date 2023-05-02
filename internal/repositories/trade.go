package repositories

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"gateway/internal/engine/types"

	deribitModel "gateway/internal/deribit/model"

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

func (r TradeRepository) Find(filter interface{}, sort interface{}, offset, limit int64) ([]*types.Trade, error) {
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

	Trades := []*types.Trade{}

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
) ([]*deribitModel.DeribitGetUserTradesByInstrumentsResponse, error) {
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
		bson.M{
			"$lookup": bson.M{
				"from":         "orders",
				"localField":   "takerOrderId",
				"foreignField": "_id",
				"as":           "takerOrder",
			},
		},
		bson.M{
			"$lookup": bson.M{
				"from":         "orders",
				"localField":   "makerOrderId",
				"foreignField": "_id",
				"as":           "makerOrder",
			},
		},
		bson.M{"$limit": count},
		bson.M{
			"$match": bson.M{
				"underlying":  _underlying,
				"strikePrice": _strikePrice,
				"expiryDate":  _expiryDate,
				"$or":         []bson.M{{"takerId": userId}, {"makerId": userId}},
			},
		},
		bson.M{
			"$project": bson.M{
				"amount":    "$amount",
				"direction": "$side",
				"order_id": bson.M{
					"$cond": bson.A{
						fmt.Sprintf("$takerId == %s", userId),
						"$takerOrderId",
						"$makerOrderId",
					},
				},
				"order_type": bson.M{
					"$cond": bson.A{
						fmt.Sprintf("$takerId == %s", userId),
						bson.M{"$arrayElemAt": bson.A{"$takerOrderId.type", 0}},
						bson.M{"$arrayElemAt": bson.A{"$makerOrder.type", 0}},
					},
				},
				"price": "$price",
				"state": bson.M{
					"$cond": bson.A{
						fmt.Sprintf("$takerId == %s", userId),
						bson.M{"$arrayElemAt": bson.A{"$takerOrderId.status", 0}},
						bson.M{"$arrayElemAt": bson.A{"$makerOrder.status", 0}},
					},
				},
			},
		},
	}

	cursor, err := r.collection.Aggregate(context.Background(), query, &options)
	if err != nil {
		return nil, err
	}

	defer cursor.Close(context.Background())

	trades := []*deribitModel.DeribitGetUserTradesByInstrumentsResponse{}

	if err = cursor.All(context.Background(), &trades); err != nil {
		return nil, err
	}

	return trades, nil
}
