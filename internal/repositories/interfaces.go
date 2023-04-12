package repositories

import "go.mongodb.org/mongo-driver/mongo"

type Database interface {
	InitCollection(collectionName string) *mongo.Collection
}
