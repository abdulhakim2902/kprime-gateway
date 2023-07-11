package schema

import (
	"github.com/Undercurrent-Technologies/kprime-utilities/models/order"
	"github.com/Undercurrent-Technologies/kprime-utilities/types"
	"github.com/hashicorp/go-memdb"
)

type User struct {
	ID              string                 `json:"id"`
	OrderExclusions []order.OrderExclusion `json:"order_exclusions"`
	TypeInclusions  []order.TypeInclusions `json:"type_inclustion"`
	ClientIds       []string               `json:"client_ids"`
	Role            types.UserRole         `json:"role"`
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
				"role": {
					Name:    "role",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "Role"},
				},
			},
		},
	},
}
