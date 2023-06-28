package service

import (
	"context"
	"fmt"
	"gateway/internal/repositories"
	"gateway/internal/user/types"
	"gateway/schema"
	"net/http"

	"gateway/pkg/memdb"
	"gateway/pkg/middleware/api"
	"gateway/pkg/utils"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/models/order"
	utilType "git.devucc.name/dependencies/utilities/types"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type userService struct {
	r *gin.Engine

	repo  *repositories.UserRepository
	memDb *memdb.Schemas
}

func NewUserService(
	r *gin.Engine,

	repo *repositories.UserRepository,
	memDb *memdb.Schemas,
) IUserService {
	svc := userService{r, repo, memDb}
	svc.RegisterRoutes()

	return &svc
}

func (svc *userService) RegisterRoutes() {
	internalAPI := svc.r.Group("api/internal")
	internalAPI.Use(api.IPWhitelist(), api.BasicAuth())

	internalAPI.POST("/sync/:target", svc.handleSync)
}

// @BasePath /api/internal

// Sync memdb with mongodb godoc
// @Summary Sync memdb with mongodb
// @Schemes
// @Description do sync
// @Tags internal
// @Accept json
// @Produce json
// @Success 200 {string} success
// @Param target path string true "target"
// @Router /sync/{target} [post]
func (svc *userService) handleSync(c *gin.Context) {
	switch c.Param("target") {
	case "users":
		svc.syncMemDB(c)
	default:
		c.AbortWithStatus(http.StatusNotFound)
	}
}

func (svc *userService) syncMemDB(c *gin.Context) {

	type Request struct {
		UserIds []string `json:"user_ids" validate:"required"`
	}

	var req Request
	if err := utils.UnmarshalAndValidate(c, &req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ids := make([]primitive.ObjectID, 0)
	for _, id := range req.UserIds {
		objId, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s is not a valid user id", id)})
			return
		}
		ids = append(ids, objId)
	}

	if err := svc.SyncMemDB(
		c.Request.Context(),
		bson.D{{"_id", bson.D{{"$in", ids}}}},
	); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})

}

func (svc *userService) SyncMemDB(ctx context.Context, filter interface{}) (err error) {
	logs.Log.Info().Msg("Sync user to memdb...")

	var users []*types.User

	users, err = svc.repo.Find(filter, nil, 0, -1)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		return
	}

	for _, user := range users {

		var typeInclusions []order.TypeInclusions
		for _, orderType := range user.OrderTypes {
			typeInclusions = append(typeInclusions, order.TypeInclusions{
				Name: orderType.Name,
			})
		}

		var orderExclusions []order.OrderExclusion
		for _, item := range user.OrderExclusions {
			orderExclusions = append(orderExclusions, order.OrderExclusion{
				UserID: item.UserID,
			})
		}

		var clientIds []string
		for _, client := range user.APICredentials {
			clientIds = append(clientIds, fmt.Sprintf("%s:%s", client.APIKey, client.APISecret))
		}

		if err = svc.memDb.User.Create(schema.User{
			ID:              user.ID.Hex(),
			OrderExclusions: orderExclusions,
			TypeInclusions:  typeInclusions,
			ClientIds:       clientIds,
			Role:            utilType.UserRole(user.Role.Name),
		}); err != nil {
			logs.Log.Error().Err(err).Msg("")
		}
	}

	logs.Log.Info().Msg(fmt.Sprintf("Sync %v user has finish...", len(users)))

	return
}
