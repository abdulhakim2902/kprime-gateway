package controller

import (
	"context"
	"errors"
	"net/http"
	"time"

	deribitModel "gateway/internal/deribit/model"
	authService "gateway/internal/user/service"
	userType "gateway/internal/user/types"

	"gateway/pkg/hmac"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"
	"strings"

	"github.com/Undercurrent-Technologies/kprime-utilities/config/types"
	"github.com/Undercurrent-Technologies/kprime-utilities/types/validation_reason"

	"github.com/gin-gonic/gin"
)

func (handler *DeribitHandler) RegisterPublic() {
	handler.RegisterHandler("public/auth", handler.auth)
	handler.RegisterHandler("public/test", handler.test)
	handler.RegisterHandler("public/get_index_price", handler.getIndexPrice)
	handler.RegisterHandler("public/get_last_trades_by_instrument", handler.getLastTradesByInstrument)
	handler.RegisterHandler("public/get_delivery_prices", handler.getDeliveryPrices)
	handler.RegisterHandler("public/get_time", handler.getTime)
}

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

// auth asyncApi
// @summary authenticate user
// @description authenticate user with client_key and client_secrets
// @payload types.AuthParams
// @x-response types.AuthResponse
// @contentType application/json
// @auth public
// @queue public/auth
// @method auth
// @tags public auth
func (h *DeribitHandler) auth(r *gin.Context) {
	var msg deribitModel.RequestDto[Params]
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
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
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
		validateSignatureAuth(msg.Params, connKey)
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
		if msg.Params.GrantType == "" {
			protocol.SendValidationMsg(connKey,
				validation_reason.INVALID_PARAMS, errors.New("grant_type is a required field"))
			return
		}
		protocol.SendValidationMsg(connKey, validation_reason.INVALID_PARAMS, nil)
		return
	}

}

func validateSignatureAuth(params Params, connKey string) {
	if params.ClientID == "" {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("client_id is a required field"))
		return
	}

	if params.Signature == "" {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("signature is a required field"))
		return
	}

	if params.Timestamp == "" {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("timestamp is a required field"))
		return
	}

	if params.GrantType == "" {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("grant_type is a required field"))
		return
	}
}

func (h *DeribitHandler) getIndexPrice(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetIndexPriceParams]
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
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	if types.Pair(msg.Params.IndexName).IsValid() == false {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("invalid index_name"))
		return
	}
	// Call service
	result := h.svc.GetIndexPrice(context.TODO(), deribitModel.DeribitGetIndexPriceRequest{
		IndexName: msg.Params.IndexName,
	})

	protocol.SendSuccessMsg(connKey, result)
	return
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
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	instruments, err := utils.ParseInstruments(msg.Params.InstrumentName, false)
	if err != nil {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, err)
		return
	}

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
	return
}

func (h *DeribitHandler) getDeliveryPrices(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.DeliveryPricesParams]
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
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	if types.Pair(msg.Params.IndexName).IsValid() == false {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("invalid index_name"))
		return
	}

	result := h.svc.GetDeliveryPrices(context.TODO(), deribitModel.DeliveryPricesRequest{
		IndexName: msg.Params.IndexName,
		Offset:    msg.Params.Offset,
		Count:     msg.Params.Count,
	})

	protocol.SendSuccessMsg(connKey, result)
	return
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

func (h *DeribitHandler) getTime(r *gin.Context) {
	var msg deribitModel.RequestDto[interface{}]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}
	now := time.Now().UnixMilli()
	r.JSON(http.StatusOK, protocol.RPCResponseMessage{
		JSONRPC: "2.0",
		Result:  now,
		ID:      msg.Id,
	})
	return
}
