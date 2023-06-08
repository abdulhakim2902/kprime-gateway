package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	_deribitModel "gateway/internal/deribit/model"
	_engineType "gateway/internal/engine/types"
	_orderbookType "gateway/internal/orderbook/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SettlementPriceRepository struct {
	collection *mongo.Collection
}

func NewSettlementPriceRepository(db Database) *SettlementPriceRepository {
	collection := db.InitCollection("settlement_prices")
	return &SettlementPriceRepository{collection}
}

func (r SettlementPriceRepository) Find(filter interface{}, sort interface{}, offset, limit int64) ([]*_engineType.SettlementPrice, error) {
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

	SettlementPrices := []*_engineType.SettlementPrice{}

	err = cursor.All(context.Background(), &SettlementPrices)
	if err != nil {
		return nil, err
	}

	return SettlementPrices, nil
}

func (r SettlementPriceRepository) GetLatestSettlementPrice(o _orderbookType.GetOrderBook) []*_engineType.SettlementPrice {
	metadataType := "index"
	metadataPair := fmt.Sprintf("%s_usd", strings.ToLower(o.Underlying))

	tradesQuery := bson.M{
		"metadata.pair": metadataPair,
		"metadata.type": metadataType,
	}
	tradesSort := bson.M{
		"ts": -1,
	}

	trades, err := r.Find(tradesQuery, tradesSort, 0, 1)
	if err != nil {
		panic(err)
	}

	return trades
}

func (r SettlementPriceRepository) GetDeliveryPrice(o _deribitModel.DeliveryPricesRequest) (_deribitModel.DeliveryPricesResponse, error) {
	matchStage := bson.D{
		{"$match", bson.D{
			{"metadata.pair", o.IndexName},
		}},
	}

	projectStage := bson.D{
		{"$project", bson.D{
			{"date", bson.D{
				{"$dateToString", bson.D{
					{"format", "%Y-%m-%d"},
					{"date", "$ts"},
				}},
			}},
			{"delivery_price", "$price"},
			{"_id", 0},
		}},
	}

	pipeline := mongo.Pipeline{matchStage, projectStage}

	if o.Offset > 0 {
		skipStage := bson.D{
			{"$skip", o.Offset},
		}
		pipeline = append(pipeline, skipStage)
	}

	if o.Count > 0 {
		limitStage := bson.D{
			{"$limit", o.Count},
		}
		pipeline = append(pipeline, limitStage)
	}

	groupStage := bson.D{
		{"$group", bson.D{
			{"_id", nil},
			{"records_total", bson.D{{"$sum", 1}}},
			{"prices", bson.D{{"$push", "$$ROOT"}}},
		}},
	}
	pipeline = append(pipeline, groupStage)

	options := options.Aggregate().SetMaxTime(10 * time.Second)

	cursor, err := r.collection.Aggregate(context.Background(), pipeline, options)
	if err != nil {
		return _deribitModel.DeliveryPricesResponse{}, err
	}
	defer cursor.Close(context.Background())

	var result _deribitModel.DeliveryPricesResponse
	if cursor.Next(context.Background()) {
		err := cursor.Decode(&result)
		if err != nil {
			return _deribitModel.DeliveryPricesResponse{}, err
		}
	}

	if err := cursor.Err(); err != nil {
		return _deribitModel.DeliveryPricesResponse{}, err
	}

	return result, nil
}
