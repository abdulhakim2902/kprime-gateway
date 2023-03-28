package controller

import (
	"gateway/internal/deribit/service"
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

	r.POST("deribit/parser", handler.DeribitParseOrder)
}

func (h DeribitHandler) DeribitParseOrder(r *gin.Context) {
	parser, err := h.svc.DeribitParseOrder(r.Request.Context(), r.Request.URL.Query())
	if err != nil {
		r.JSON(http.StatusInternalServerError, err)
		return
	}
	r.JSON(http.StatusOK, parser)
}
