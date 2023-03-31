package controller

import (
	"encoding/json"
	_deribitModel "gateway/internal/deribit/model"
	"gateway/internal/deribit/service"
	"gateway/pkg/model"
	"net/http"

	"github.com/gin-gonic/gin"
)

type DeribitHandler struct {
	svc service.IDeribitService
}

func NewDeribitHandler(r *gin.Engine, svc service.IDeribitService) {
	handler := &DeribitHandler{
		svc: svc,
	}

	r.POST("private/buy", handler.DeribitParseBuy)
	r.POST("private/sell", handler.DeribitParseSell)
}

func (h DeribitHandler) DeribitParseBuy(r *gin.Context) {
	var req _deribitModel.DeribitRequest
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	order, err := h.svc.DeribitParseBuy(r.Request.Context(), req)
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

func (h DeribitHandler) DeribitParseSell(r *gin.Context) {
	var req _deribitModel.DeribitRequest
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	order, err := h.svc.DeribitParseSell(r.Request.Context(), req)
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
