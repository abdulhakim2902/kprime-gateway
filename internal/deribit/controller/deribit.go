package controller

import (
	"encoding/json"
	"fmt"
	_deribitModel "gateway/internal/deribit/model"
	"gateway/internal/deribit/service"
	"gateway/pkg/model"
	"gateway/pkg/rbac/middleware"
	validator "github.com/go-playground/validator/v10"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Create a new instance of the validator
var validate = validator.New()

type DeribitHandler struct {
	svc service.IDeribitService
}

func NewDeribitHandler(r *gin.Engine, svc service.IDeribitService) {
	handler := &DeribitHandler{
		svc: svc,
	}

	private := r.Group("/private")
	public := r.Group("/api/v2/public")

	private.POST("buy", middleware.Authenticate(), handler.DeribitParseBuy)
	private.POST("sell", middleware.Authenticate(), handler.DeribitParseSell)
	private.POST("edit", middleware.Authenticate(), handler.DeribitParseEdit)
	private.POST("cancel", middleware.Authenticate(), handler.DeribitParseCancel)
	public.GET("test", handler.DeribitTest)
}

func (h DeribitHandler) DeribitParseBuy(r *gin.Context) {
	// Get user ID
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

	// Get request body
	var req _deribitModel.DeribitRequest
	errJson := json.NewDecoder(r.Request.Body).Decode(&req)
	if errJson != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: errJson.Error(),
		})
		return
	}

	// Perform validation
	if err := validate.Struct(req); err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	// Call service
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
	// Get user ID
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

	// Get request body
	var req _deribitModel.DeribitRequest
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	// Perform validation
	if err := validate.Struct(req); err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	// Call service
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

func (h DeribitHandler) DeribitParseEdit(r *gin.Context) {
	// Get user ID
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

	// Get request body
	var req _deribitModel.DeribitEditRequest
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	// Perform validation
	if err := validate.Struct(req); err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	// Call service
	order, err := h.svc.DeribitParseEdit(r.Request.Context(), userIDStr, req)
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

func (h DeribitHandler) DeribitParseCancel(r *gin.Context) {
	// Get user ID
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

	// Get request body
	var req _deribitModel.DeribitCancelRequest
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	// Perform validation
	if err := validate.Struct(req); err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	// Call service
	order, err := h.svc.DeribitParseCancel(r.Request.Context(), userIDStr, req)
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

func (h DeribitHandler) DeribitTest(r *gin.Context) {
	r.JSON(http.StatusAccepted, gin.H{
		"jsonrpc": "2.0",
		"result": gin.H{
			"version": "1.2.26",
		},
		"testnet": true,
		"usIn":    0,
		"usOut":   0,
		"usDiff":  0,
	})
}
