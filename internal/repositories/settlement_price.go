package repositories

import (
	"context"

	_engineType "gateway/internal/engine/types"

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
