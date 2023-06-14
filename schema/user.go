package schema

import (
	"git.devucc.name/dependencies/utilities/models/order"
	"github.com/hashicorp/go-memdb"
)

type User struct {
	ID              string                 `json:"id"`
	OrderExclusions []order.OrderExclusion `json:"order_exclusions"`
	TypeInclusions  []order.TypeInclusions `json:"type_inclustion"`
	ClientIds       []string               `json:"client_ids"`
}

var UserSchema = &memdb.DBSchema{
	Tables: map[string]*memdb.TableSchema{
		"user": {
			Name: "user",
			Indexes: map[string]*memdb.IndexSchema{
				"id": {
					Name:    "id",
					Unique:  true,
					Indexer: &memdb.StringFieldIndex{Field: "ID"},
				},
				"order_exclusions": {
					Name:    "order_exclusions",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "OrderExclusions"},
				},
				"type_inclustion": {
					Name:    "type_inclustion",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "TypeInclusions"},
				},
				"client_ids": {
					Name:    "client_ids",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "ClientIds"},
				},
			},
		},
	},
}
