package controller

import (
	"encoding/json"
	"fmt"
	_deribitModel "gateway/internal/deribit/model"
	"gateway/internal/deribit/service"
	"gateway/pkg/model"
	"gateway/pkg/rbac/middleware"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type DeribitHandler struct {
	svc service.IDeribitService
}

func NewDeribitHandler(r *gin.Engine, svc service.IDeribitService) {
	handler := &DeribitHandler{
		svc: svc,
	}

	r.POST("private/buy", middleware.Authenticate(), handler.DeribitParseBuy)
	r.POST("private/sell", middleware.Authenticate(), handler.DeribitParseSell)
}

func (h DeribitHandler) DeribitParseBuy(r *gin.Context) {
	userID, err := r.Get("userID")
	if !err {
		fmt.Println(err)
		r.JSON(http.StatusBadRequest, &model.Response{
			Error: true,
		})
		return
	}

	// Convert to float
	userIDFloat, ok := userID.(float64)
	if !ok {
		fmt.Println("!ok")

		r.JSON(http.StatusBadRequest, &model.Response{
			Error: true,
		})
		return
	}

	// Convert to string
	userIDStr := strconv.FormatFloat(userIDFloat, 'f', 0, 64)

	var req _deribitModel.DeribitRequest
	errJson := json.NewDecoder(r.Request.Body).Decode(&req)
	if errJson != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: errJson.Error(),
		})
		return
	}

	order, errJson := h.svc.DeribitParseBuy(r.Request.Context(), userIDStr, req)
	if errJson != nil {
		r.JSON(http.StatusInternalServerError, &model.Response{
			Error:   true,
			Message: errJson.Error(),
		})
		return
	}

	r.JSON(http.StatusAccepted, &model.Response{
		Data: order,
	})
}

func (h DeribitHandler) DeribitParseSell(r *gin.Context) {

	userID, errGin := r.Get("userID")
	if !errGin {
		fmt.Println(errGin)
		r.JSON(http.StatusBadRequest, &model.Response{
			Error: true,
		})
		return
	}

	// Convert to float
	userIDFloat, ok := userID.(float64)
	if !ok {
		fmt.Println("!ok")

		r.JSON(http.StatusBadRequest, &model.Response{
			Error: true,
		})
		return
	}

	// Convert to string
	userIDStr := strconv.FormatFloat(userIDFloat, 'f', 0, 64)

	var req _deribitModel.DeribitRequest
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	order, err := h.svc.DeribitParseSell(r.Request.Context(), userIDStr, req)
	if err != nil {
		r.JSON(http.StatusInternalServerError, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	r.JSON(http.StatusAccepted, &model.Response{
		Data: order,
	})
}
