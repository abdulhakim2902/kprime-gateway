package controller

import (
	"context"
	"errors"
	"fmt"
	"gateway/internal/deribit/service"
	"net/http"
	"strconv"
	"strings"
	"time"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/types"
	"go.mongodb.org/mongo-driver/bson/primitive"

	deribitModel "gateway/internal/deribit/model"
	_engineType "gateway/internal/engine/types"
	authService "gateway/internal/user/service"
	userType "gateway/internal/user/types"

	"gateway/pkg/hmac"
	"gateway/pkg/memdb"
	"gateway/pkg/middleware"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"

	"git.devucc.name/dependencies/utilities/types/validation_reason"
	cors "github.com/rs/cors/wrapper/gin"

	"gateway/internal/repositories"

	"github.com/gin-gonic/gin"
)

type DeribitHandler struct {
	svc      service.IDeribitService
	authSvc  authService.IAuthService
	userRepo *repositories.UserRepository
	memDb    *memdb.Schemas

	handlers map[string]gin.HandlerFunc
}

func NewDeribitHandler(
	r *gin.Engine,
	svc service.IDeribitService,
	authSvc authService.IAuthService,
	userRepo *repositories.UserRepository,
	memDb *memdb.Schemas,
) {
	handler := DeribitHandler{
		svc:      svc,
		authSvc:  authSvc,
		userRepo: userRepo,
		memDb:    memDb,
	}

	r.Use(cors.AllowAll())
	r.Use(middleware.Authenticate(memDb))

	api := r.Group("/api/v2")
	api.POST("", handler.ApiPostHandler)
	api.GET(":type/*action", handler.ApiGetHandler)

	handler.RegisterPrivate()
	handler.RegisterPublic()
}

func (h *DeribitHandler) RegisterHandler(method string, handler gin.HandlerFunc) {
	if h.handlers == nil {
		h.handlers = make(map[string]gin.HandlerFunc)
	}

	h.handlers[method] = handler
}

func (h *DeribitHandler) ApiPostHandler(r *gin.Context) {
	type Params struct{}

	var dto deribitModel.RequestDto[Params]
	if err := utils.UnmarshalAndValidate(r, &dto); err != nil {
		r.AbortWithStatusJSON(http.StatusBadRequest, err.Error())
		return
	}

	handler, ok := h.handlers[dto.Method]
	if !ok {
		r.AbortWithStatus(http.StatusNotFound)
		return
	}

	logs.Log.Info().Str("called_method", dto.Method).Msg("")

	handler(r)
}

func (h *DeribitHandler) ApiGetHandler(r *gin.Context) {
	method := fmt.Sprintf("%s%s", r.Param("type"), r.Param("action"))

	handler, ok := h.handlers[method]
	if !ok {
		r.AbortWithStatus(http.StatusNotFound)
		return
	}

	logs.Log.Info().Str("called_method", method).Msg("")

	handler(r)
}

func requestHelper(msgID uint64, method string, c *gin.Context) (
	userId,
	connKey string,
	reason *validation_reason.ValidationReason,
	err error,
) {
	key := utils.GetKeyFromIdUserID(msgID, "")
	if isDuplicateConnection := protocol.RegisterProtocolRequest(
		key, protocol.ProtocolRequest{Http: c, Protocol: protocol.HTTP, Method: method},
	); isDuplicateConnection {
		validation := validation_reason.DUPLICATED_REQUEST_ID
		reason = &validation

		err = errors.New(validation.String())
		return
	}

	userId = c.GetString("userID")

	if len(userId) == 0 {
		connKey = key
		return
	}

	connKey = utils.GetKeyFromIdUserID(msgID, userId)
	protocol.UpgradeProtocol(key, connKey)

	return
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

func (h *DeribitHandler) buy(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	maxShow := 0.1
	if msg.Params.MaxShow == nil {
		msg.Params.MaxShow = &maxShow
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	if err := utils.ValidateDeribitRequestParam(msg.Params); err != nil {
		protocol.SendValidationMsg(connKey, validation_reason.INVALID_PARAMS, err)
		return
	}

	// Call service
	order, validation, err := h.svc.DeribitRequest(r.Request.Context(), userID, deribitModel.DeribitRequest{
		InstrumentName: msg.Params.InstrumentName,
		Amount:         msg.Params.Amount,
		Type:           msg.Params.Type,
		Price:          msg.Params.Price,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
		TimeInForce:    msg.Params.TimeInForce,
		Label:          msg.Params.Label,
		Side:           types.BUY,
		MaxShow:        *msg.Params.MaxShow,
		ReduceOnly:     msg.Params.ReduceOnly,
		PostOnly:       msg.Params.PostOnly,
	})
	if err != nil {
		if validation != nil {
			protocol.SendValidationMsg(connKey, *validation, err)
			return
		}

		protocol.SendErrMsg(connKey, err)
		return
	}

	orderId, _ := primitive.ObjectIDFromHex(order.ID)
	response := _engineType.BuySellEditResponse{
		Order: _engineType.BuySellEditCancelOrder{
			OrderState:          types.OrderStatus(order.Status),
			Usd:                 order.Price,
			FilledAmount:        order.FilledAmount,
			InstrumentName:      order.Underlying + "-" + order.ExpirationDate + "-" + fmt.Sprintf("%.0f", order.StrikePrice) + "-" + string(order.Contracts[0]),
			Direction:           types.Side(order.Side),
			LastUpdateTimestamp: utils.MakeTimestamp(order.CreatedAt),
			Price:               order.Price,
			Amount:              order.Amount,
			OrderId:             orderId,
			OrderType:           types.Type(order.Type),
			TimeInForce:         types.TimeInForce(order.TimeInForce),
			CreationTimestamp:   utils.MakeTimestamp(order.CreatedAt),
			Label:               order.Label,
			Api:                 true,
			AveragePrice:        0,
			MaxShow:             order.MaxShow,
			PostOnly:            order.PostOnly,
			ReduceOnly:          order.ReduceOnly,
		},
		Trades: []_engineType.BuySellEditTrade{},
	}

	protocol.SendSuccessMsg(connKey, response)
}

func (h *DeribitHandler) sell(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	maxShow := 0.1
	if msg.Params.MaxShow == nil {
		msg.Params.MaxShow = &maxShow
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	if err := utils.ValidateDeribitRequestParam(msg.Params); err != nil {
		protocol.SendValidationMsg(connKey, validation_reason.INVALID_PARAMS, err)
		return
	}

	// Call service
	order, validation, err := h.svc.DeribitRequest(r.Request.Context(), userID, deribitModel.DeribitRequest{
		InstrumentName: msg.Params.InstrumentName,
		Amount:         msg.Params.Amount,
		Type:           msg.Params.Type,
		Price:          msg.Params.Price,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
		TimeInForce:    msg.Params.TimeInForce,
		Label:          msg.Params.Label,
		Side:           types.SELL,
		MaxShow:        *msg.Params.MaxShow,
		ReduceOnly:     msg.Params.ReduceOnly,
		PostOnly:       msg.Params.PostOnly,
	})
	if err != nil {
		if validation != nil {
			protocol.SendValidationMsg(connKey, *validation, err)
			return
		}

		protocol.SendErrMsg(connKey, err)
		return
	}

	orderId, _ := primitive.ObjectIDFromHex(order.ID)
	response := _engineType.BuySellEditResponse{
		Order: _engineType.BuySellEditCancelOrder{
			OrderState:          types.OrderStatus(order.Status),
			Usd:                 order.Price,
			FilledAmount:        order.FilledAmount,
			InstrumentName:      order.Underlying + "-" + order.ExpirationDate + "-" + fmt.Sprintf("%.0f", order.StrikePrice) + "-" + string(order.Contracts[0]),
			Direction:           types.Side(order.Side),
			LastUpdateTimestamp: utils.MakeTimestamp(order.CreatedAt),
			Price:               order.Price,
			Amount:              order.Amount,
			OrderId:             orderId,
			OrderType:           types.Type(order.Type),
			TimeInForce:         types.TimeInForce(order.TimeInForce),
			CreationTimestamp:   utils.MakeTimestamp(order.CreatedAt),
			Label:               order.Label,
			Api:                 true,
			AveragePrice:        0,
			MaxShow:             order.MaxShow,
			PostOnly:            order.PostOnly,
			ReduceOnly:          order.ReduceOnly,
		},
		Trades: []_engineType.BuySellEditTrade{},
	}

	protocol.SendSuccessMsg(connKey, response)
}

func (h *DeribitHandler) edit(r *gin.Context) {

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}
	// Call service
	order, err := h.svc.DeribitParseEdit(r.Request.Context(), userID, deribitModel.DeribitEditRequest{
		Id:      msg.Params.Id,
		Price:   msg.Params.Price,
		Amount:  msg.Params.Amount,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}

	protocol.SendSuccessMsg(connKey, order)
}

func (h *DeribitHandler) cancel(r *gin.Context) {

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Call service
	order, err := h.svc.DeribitParseCancel(r.Request.Context(), userID, deribitModel.DeribitCancelRequest{
		Id:      msg.Params.Id,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}

	protocol.SendSuccessMsg(connKey, order)
}

func (h *DeribitHandler) cancelByInstrument(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Call service
	order, err := h.svc.DeribitCancelByInstrument(r.Request.Context(), userID, deribitModel.DeribitCancelByInstrumentRequest{
		InstrumentName: msg.Params.InstrumentName,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}

	protocol.SendSuccessMsg(connKey, order)
}

func (h *DeribitHandler) cancelAll(r *gin.Context) {

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Call service
	order, err := h.svc.DeribitParseCancelAll(r.Request.Context(), userID, deribitModel.DeribitCancelAllRequest{
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}

	protocol.SendSuccessMsg(connKey, order)
}

func (h *DeribitHandler) getUserTradeByInstrument(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetUserTradesByInstrumentParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Number of requested items, default - 10
	if msg.Params.Count <= 0 {
		msg.Params.Count = 10
	}

	res := h.svc.DeribitGetUserTradesByInstrument(
		r.Request.Context(),
		userID,
		deribitModel.DeribitGetUserTradesByInstrumentsRequest{
			InstrumentName: msg.Params.InstrumentName,
			Count:          msg.Params.Count,
			StartTimestamp: msg.Params.StartTimestamp,
			EndTimestamp:   msg.Params.EndTimestamp,
			Sorting:        msg.Params.Sorting,
		},
	)

	protocol.SendSuccessMsg(connKey, res)
}
func (h *DeribitHandler) getOpenOrdersByInstrument(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetOpenOrdersByInstrumentParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// type parameter
	if msg.Params.Type == "" {
		msg.Params.Type = "all"
	}

	res := h.svc.DeribitGetOpenOrdersByInstrument(
		context.TODO(),
		userId,
		deribitModel.DeribitGetOpenOrdersByInstrumentRequest{
			InstrumentName: msg.Params.InstrumentName,
			Type:           msg.Params.Type,
		},
	)

	protocol.SendSuccessMsg(userId, res)
}

func (h *DeribitHandler) getOrderHistoryByInstrument(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetOrderHistoryByInstrumentParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// parameter default value
	if msg.Params.Count <= 0 {
		msg.Params.Count = 20
	}

	res := h.svc.DeribitGetOrderHistoryByInstrument(
		context.TODO(),
		userId,
		deribitModel.DeribitGetOrderHistoryByInstrumentRequest{
			InstrumentName:  msg.Params.InstrumentName,
			Count:           msg.Params.Count,
			Offset:          msg.Params.Offset,
			IncludeOld:      msg.Params.IncludeOld,
			IncludeUnfilled: msg.Params.IncludeUnfilled,
		},
	)

	protocol.SendSuccessMsg(connKey, res)
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
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
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

func (h *DeribitHandler) getOrderStateByLabel(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.DeribitGetOrderStateByLabelRequest]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	msg.Params.UserId = userId

	res := h.svc.DeribitGetOrderStateByLabel(r.Request.Context(), msg.Params)

	protocol.SendSuccessMsg(connKey, res)
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

func (h *DeribitHandler) getOrderState(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetOrderStateParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Call service
	res := h.svc.DeribitGetOrderState(
		context.TODO(),
		userId,
		deribitModel.DeribitGetOrderStateRequest{
			OrderId: msg.Params.OrderId,
		},
	)

	protocol.SendSuccessMsg(connKey, res)
}

func (h *DeribitHandler) getUserTradesByOrder(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetUserTradesByOrderParams]

	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	res := h.svc.DeribitGetUserTradesByOrder(
		context.TODO(),
		userId,
		deribitModel.DeribitGetUserTradesByOrderRequest{
			OrderId: msg.Params.OrderId,
			Sorting: msg.Params.Sorting,
		},
	)

	protocol.SendSuccessMsg(connKey, res)
}

func (h *DeribitHandler) getAccountSummary(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetAccountSummary]

	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	result := h.svc.FetchUserBalance(
		msg.Params.Currency,
		userId,
	)
	balance, _ := strconv.ParseFloat(result.Balance, 64)

	user, _ := h.userRepo.FindById(context.TODO(), userId)
	resp := deribitModel.GetAccountSummaryResponse{
		Id:                userId,
		Currency:          msg.Params.Currency,
		Email:             user.Email,
		Balance:           balance,
		MarginBalance:     balance,
		CreationTimestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}

	protocol.SendSuccessMsg(connKey, resp)
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
