package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	_engineType "gateway/internal/engine/types"
	_orderbookType "gateway/internal/orderbook/types"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type RawPriceRepository struct {
	collection *mongo.Collection
}

func NewRawPriceRepository(db Database) *RawPriceRepository {
	collection := db.InitCollection("raw_prices")
	return &RawPriceRepository{collection}
}

func (r RawPriceRepository) Find(filter interface{}, sort interface{}, offset, limit int64) ([]*_engineType.RawPrice, error) {
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

	RawPrice := []*_engineType.RawPrice{}

	err = cursor.All(context.Background(), &RawPrice)
	if err != nil {
		return nil, err
	}

	return RawPrice, nil
}

func (r RawPriceRepository) GetLatestIndexPrice(o _orderbookType.GetOrderBook) []*_engineType.RawPrice {
	metadataType := "index"
	metadataPair := fmt.Sprintf("%s_usd", strings.ToLower(o.Underlying))

	gteDate := 60
	gteTs := time.Now().Add(-1 * (time.Second * time.Duration(gteDate)))

	tradesQuery := bson.M{
		"metadata.pair": metadataPair,
		"metadata.type": metadataType,
		"ts": bson.M{
			"$gte": gteTs,
		},
	}
	tradesSort := bson.M{
		"ts": -1,
	}

	trades, err := r.Find(tradesQuery, tradesSort, 0, 1)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		return nil
	}

	return trades
}

func (r RawPriceRepository) GetIndexPrice(indexName string) []*_engineType.RawPrice {
	metadataType := "index"
	metadataPair := strings.ToLower(indexName)

	tradesQuery := bson.M{
		"metadata.pair": metadataPair,
		"metadata.type": metadataType,
	}
	tradesSort := bson.M{
		"ts": -1,
	}

	trades, err := r.Find(tradesQuery, tradesSort, 0, 1)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		return nil
	}

	return trades
}
