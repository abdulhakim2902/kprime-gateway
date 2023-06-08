package server

import (
	i "git.devucc.name/dependencies/utilities/helper/initialize"
	"git.devucc.name/dependencies/utilities/repository/memdb"
	"git.devucc.name/dependencies/utilities/repository/mongodb"
)

var groupID = "gateway-group"

func InitializeData(r *mongodb.Repositories, mr *memdb.Repositories) {
	initialize := i.NewInitialize(r, mr)
	initialize.System()
}
