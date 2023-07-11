package server

import (
	i "github.com/Undercurrent-Technologies/kprime-utilities/helper/initialize"
	"github.com/Undercurrent-Technologies/kprime-utilities/repository/memdb"
	"github.com/Undercurrent-Technologies/kprime-utilities/repository/mongodb"
)

var groupID = "gateway-group"

func InitializeData(r *mongodb.Repositories, mr *memdb.Repositories) {
	initialize := i.NewInitialize(r, mr)
	initialize.System()
}
