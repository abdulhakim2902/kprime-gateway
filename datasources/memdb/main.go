package memdb

import (
	"github.com/hashicorp/go-memdb"
)

var Database *memdb.MemDB

func InitSchema(schema *memdb.DBSchema) error {
	db, err := memdb.NewMemDB(schema)
	if err != nil {
		return err
	}

	Database = db

	return nil
}
