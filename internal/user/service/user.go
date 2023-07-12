package service

import (
	"context"
	"fmt"
	"gateway/internal/repositories"
	"gateway/internal/user/types"
	"gateway/schema"
	"net/http"
	"os"
	"time"

	"gateway/pkg/memdb"
	"gateway/pkg/middleware/api"
	"gateway/pkg/utils"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"github.com/Undercurrent-Technologies/kprime-utilities/models/order"
	utilType "github.com/Undercurrent-Technologies/kprime-utilities/types"
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type userService struct {
	r *gin.Engine

	repo *repositories.UserRepository
}

type Request struct {
	UserIds []string `json:"user_ids" validate:"required"`
}

func NewUserService(
	r *gin.Engine,

	repo *repositories.UserRepository,
) IUserService {
	svc := userService{r, repo}
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
// @Param Request body Request true "request body"
// @Param target path string true "target entity to sync, users"
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
	var req Request
	if err := utils.UnmarshalAndValidate(c, &req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ids := make([]primitive.ObjectID, 0)
	matchingEngineUrl, ok := os.LookupEnv("MATCHING_ENGINE_URL")
	if !ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "MATCHING_ENGINE_URL is not set"})
		return
	}
	for _, id := range req.UserIds {
		objId, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s is not a valid user id", id)})
			return
		}
		ids = append(ids, objId)
		client := &http.Client{}
		url := fmt.Sprintf(`%v/api/v1/sync/user/%v`, matchingEngineUrl, id)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			logs.Log.Err(err).Msg("Error creating HTTP request!")
			return
		}

		token, _ := os.LookupEnv("PROTECT_BASIC_ENGINE")
		if token != "*" {
			auth := fmt.Sprintf("Basic %s", token)
			req.Header.Add("Authorization", auth)
		}
		res, err := client.Do(req)
		if err != nil || res.Status != "200 OK" {
			if err != nil {
				logs.Log.Error().Err(err).Msg(err.Error())
			}
			logs.Log.Error().Msg(fmt.Sprintf("%s id failed to sync to engine", id))
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s id failed to sync to engine", id)})
			return
		}
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
	start := time.Now()

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

		if err = memdb.Schemas.User.Create(schema.User{
			ID:              user.ID.Hex(),
			OrderExclusions: orderExclusions,
			TypeInclusions:  typeInclusions,
			Role:            utilType.UserRole(user.Role.Name),
		}); err != nil {
			logs.Log.Error().Err(err).Msg("")
			return
		}

		for _, client := range user.APICredentials {
			go func(cred *types.APICredentials, userId string) {
				id, _ := uuid.GenerateUUID()
				if err = memdb.Schemas.UserCredential.Create(schema.UserCredential{
					ID:     id,
					UserID: userId,
					Key:    cred.APIKey,
					Secret: cred.APISecret,
				}); err != nil {
					logs.Log.Error().Err(err).Msg("")
					return
				}
			}(client, user.ID.Hex())
		}
	}

	logs.Log.Info().Msg(fmt.Sprintf("Sync %d user has finished, took %v", len(users), time.Since(start)))

	return
}
