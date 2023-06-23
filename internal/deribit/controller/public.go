package controller

import (
	"context"
	"errors"
	"net/http"

	deribitModel "gateway/internal/deribit/model"
	authService "gateway/internal/user/service"
	userType "gateway/internal/user/types"

	"gateway/pkg/hmac"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"
	"strings"

	"git.devucc.name/dependencies/utilities/types/validation_reason"

	"github.com/gin-gonic/gin"
)

func (handler *DeribitHandler) RegisterPublic() {
	handler.RegisterHandler("public/auth", handler.auth)
	handler.RegisterHandler("public/get_instruments", handler.getInstruments)
	handler.RegisterHandler("public/get_order_book", handler.getOrderBook)
	handler.RegisterHandler("public/test", handler.test)
	handler.RegisterHandler("public/get_index_price", handler.getIndexPrice)
	handler.RegisterHandler("public/get_last_trades_by_instrument", handler.getLastTradesByInstrument)
	handler.RegisterHandler("public/get_delivery_prices", handler.getDeliveryPrices)
	handler.RegisterHandler("public/get_tradingview_chart_data", handler.publicGetTradingviewChartData)
}

func (h *DeribitHandler) auth(r *gin.Context) {
	type Params struct {
		GrantType    string `json:"grant_type" form:"grant_type"`
		ClientID     string `json:"client_id" form:"client_id"`
		ClientSecret string `json:"client_secret" form:"client_secret"`
		RefreshToken string `json:"refresh_token" form:"refresh_token"`

		Signature string `json:"signature" form:"signature"`
		Timestamp string `json:"timestamp" form:"timestamp"`
		Nonce     string `json:"nonce" form:"nonce"`
		Data      string `json:"data" form:"data"`
	}

	var msg deribitModel.RequestDto[Params]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	switch msg.Params.GrantType {
	case "client_credentials":
		payload := userType.AuthRequest{
			APIKey:    msg.Params.ClientID,
			APISecret: msg.Params.ClientSecret,
		}

		if payload.APIKey == "" || payload.APISecret == "" {
			protocol.SendValidationMsg(connKey,
				validation_reason.INVALID_PARAMS, errors.New("required client_id and client_secret"))
			return
		}

		res, _, err := h.authSvc.Login(context.TODO(), payload)
		if err != nil {
			if strings.Contains(err.Error(), "invalid credential") {
				protocol.SendValidationMsg(connKey, validation_reason.UNAUTHORIZED, err)
				return
			}

			protocol.SendErrMsg(connKey, err)
			return
		}

		protocol.SendSuccessMsg(connKey, res)
		return
	case "client_signature":
		sig := hmac.Signature{
			Ts:       msg.Params.Timestamp,
			Sig:      msg.Params.Signature,
			Nonce:    msg.Params.Nonce,
			Data:     msg.Params.Data,
			ClientId: msg.Params.ClientID,
		}
		res, _, err := h.authSvc.LoginWithSignature(context.TODO(), sig)
		if err != nil {
			if strings.Contains(err.Error(), "invalid credential") {
				protocol.SendValidationMsg(connKey, validation_reason.UNAUTHORIZED, err)
				return
			}

			protocol.SendErrMsg(connKey, err)
			return
		}

		protocol.SendSuccessMsg(connKey, res)
		return
	case "refresh_token":
		if msg.Params.RefreshToken == "" {
			protocol.SendValidationMsg(connKey,
				validation_reason.INVALID_PARAMS, errors.New("required refresh_token"))
			return
		}

		claim, err := authService.ClaimJWT(nil, msg.Params.RefreshToken)
		if err != nil {
			protocol.SendValidationMsg(connKey, validation_reason.UNAUTHORIZED, err)
			return
		}

		res, _, err := h.authSvc.RefreshToken(context.TODO(), claim)
		if err != nil {
			protocol.SendErrMsg(connKey, err)
			return
		}

		protocol.SendSuccessMsg(connKey, res)
		return
	default:
		protocol.SendValidationMsg(connKey, validation_reason.INVALID_PARAMS, nil)
		return
	}

}

func (h *DeribitHandler) getInstruments(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetInstrumentsParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		errMsg := protocol.ErrorMessage{
			Message:        err.Error(),
			Data:           protocol.ReasonMessage{},
			HttpStatusCode: http.StatusBadRequest,
		}
		m := protocol.RPCResponseMessage{
			JSONRPC: "2.0",
			ID:      msg.Id,
			Error:   &errMsg,
			Testnet: true,
		}
		r.AbortWithStatusJSON(http.StatusBadRequest, m)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	currency := map[string]bool{"BTC": true, "ETH": true, "USDC": true}
	if _, ok := currency[strings.ToUpper(msg.Params.Currency)]; !ok {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("invalid currency"))
		return
	}

	if msg.Params.Kind != "" && strings.ToLower(msg.Params.Kind) != "option" {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("invalid value of kind"))
		return
	}

	if msg.Params.IncludeSpots {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("invalid value of include_spots"))
		return
	}

	result := h.svc.DeribitGetInstruments(context.TODO(), deribitModel.DeribitGetInstrumentsRequest{
		Currency: msg.Params.Currency,
		Expired:  msg.Params.Expired,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (h *DeribitHandler) getOrderBook(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetOrderBookParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		reason := validation_reason.PARSE_ERROR
		code, _, codeStr := reason.Code()

		errMsg := protocol.ErrorMessage{
			Message: err.Error(),
			Data: protocol.ReasonMessage{
				Reason: codeStr,
			},
			Code: code,
		}
		m := protocol.RPCResponseMessage{
			JSONRPC: "2.0",
			ID:      msg.Id,
			Error:   &errMsg,
			Testnet: true,
		}
		r.AbortWithStatusJSON(http.StatusBadRequest, m)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	instruments, _ := utils.ParseInstruments(msg.Params.InstrumentName)

	if instruments == nil {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("instrument not found"))
		return
	}

	result := h.svc.GetOrderBook(context.TODO(), deribitModel.DeribitGetOrderBookRequest{
		InstrumentName: msg.Params.InstrumentName,
		Depth:          msg.Params.Depth,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (h *DeribitHandler) getIndexPrice(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetIndexPriceParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Call service
	result := h.svc.GetIndexPrice(context.TODO(), deribitModel.DeribitGetIndexPriceRequest{
		IndexName: msg.Params.IndexName,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (h *DeribitHandler) getLastTradesByInstrument(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetLastTradesByInstrumentParams]

	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		errMsg := protocol.ErrorMessage{
			Message:        err.Error(),
			Data:           protocol.ReasonMessage{},
			HttpStatusCode: http.StatusBadRequest,
		}
		m := protocol.RPCResponseMessage{
			JSONRPC: "2.0",
			ID:      msg.Id,
			Error:   &errMsg,
			Testnet: true,
		}
		r.AbortWithStatusJSON(http.StatusBadRequest, m)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	instruments, _ := utils.ParseInstruments(msg.Params.InstrumentName)

	if instruments == nil {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("invalid instrument_name"))
		return
	}

	result := h.svc.DeribitGetLastTradesByInstrument(context.TODO(), deribitModel.DeribitGetLastTradesByInstrumentRequest{
		InstrumentName: msg.Params.InstrumentName,
		StartSeq:       msg.Params.StartSeq,
		EndSeq:         msg.Params.EndSeq,
		StartTimestamp: msg.Params.StartTimestamp,
		EndTimestamp:   msg.Params.EndTimestamp,
		Count:          msg.Params.Count,
		Sorting:        msg.Params.Sorting,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (h *DeribitHandler) getDeliveryPrices(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.DeliveryPricesParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	result := h.svc.GetDeliveryPrices(context.TODO(), deribitModel.DeliveryPricesRequest{
		IndexName: msg.Params.IndexName,
		Offset:    msg.Params.Offset,
		Count:     msg.Params.Count,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (h *DeribitHandler) publicGetTradingviewChartData(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetTradingviewChartDataRequest]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	result, err := h.svc.GetTradingViewChartData(context.TODO(), msg.Params)
	if err != nil {
		reason := validation_reason.OTHER

		protocol.SendValidationMsg(connKey, reason, err)
		return

	}

	protocol.SendSuccessMsg(connKey, result)
}

func (h *DeribitHandler) test(r *gin.Context) {
	r.JSON(http.StatusOK, gin.H{
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
