package controller

import (
	"context"
	"errors"
	"gateway/internal/deribit/service"

	"git.devucc.name/dependencies/utilities/types"

	deribitModel "gateway/internal/deribit/model"
	authService "gateway/internal/user/service"
	userType "gateway/internal/user/types"

	"gateway/pkg/middleware"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"
	"strconv"
	"strings"
	"time"

	"git.devucc.name/dependencies/utilities/types/validation_reason"
	cors "github.com/rs/cors/wrapper/gin"

	"github.com/gin-gonic/gin"
)

type DeribitHandler struct {
	svc     service.IDeribitService
	authSvc authService.IAuthService
}

func NewDeribitHandler(
	r *gin.Engine,
	svc service.IDeribitService,
	authSvc authService.IAuthService,
) {
	handler := &DeribitHandler{svc, authSvc}
	r.Use(cors.AllowAll())

	// Private
	private := r.Group("/private").Use(middleware.Authenticate())

	private.POST("buy", handler.DeribitParseBuy)
	private.POST("sell", handler.DeribitParseSell)
	private.POST("edit", handler.DeribitParseEdit)
	private.POST("cancel", handler.DeribitParseCancel)
	private.POST("cancel_all_by_instrument", handler.DeribitCancelByInstrument)
	private.POST("cancel_all", handler.DeribitCancelAll)

	private.GET(":method", handler.PrivateGetHandler)

	// Public
	public := r.Group("/api/v2/public")

	public.POST("auth", handler.Auth)
	public.GET(":method", handler.PublicGetHandler)
}

func (h DeribitHandler) Auth(r *gin.Context) {
	requestedTime := uint64(time.Now().UnixMicro())

	if ok := protocol.RegisterProtocolRequest(requestedTime, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(requestedTime, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	type Params struct {
		GrantType    string `json:"grant_type"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RefreshToken string `json:"refresh_token"`
	}

	var msg deribitModel.RequestDto[Params]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		protocol.SendValidationMsg(requestedTime, validation_reason.PARSE_ERROR, err)
		return
	}

	var res any
	var err error

	switch msg.Params.GrantType {
	case "client_credentials":
		payload := userType.AuthRequest{
			APIKey:    msg.Params.ClientID,
			APISecret: msg.Params.ClientSecret,
		}

		if payload.APIKey == "" || payload.APISecret == "" {
			protocol.SendValidationMsg(requestedTime,
				validation_reason.INVALID_PARAMS, errors.New("required client_id and client_secret"))
			return
		}

		res, err = h.authSvc.Login(context.TODO(), payload)
		if err != nil {
			if strings.Contains(err.Error(), "invalid credential") {
				protocol.SendValidationMsg(requestedTime, validation_reason.UNAUTHORIZED, err)
				return
			}

			protocol.SendErrMsg(requestedTime, err)
			return
		}
	case "refresh_token":
		if msg.Params.RefreshToken == "" {
			protocol.SendValidationMsg(requestedTime,
				validation_reason.INVALID_PARAMS, errors.New("required refresh_token"))
			return
		}

		claim, err := authService.ClaimJWT(msg.Params.RefreshToken)
		if err != nil {
			protocol.SendValidationMsg(requestedTime, validation_reason.UNAUTHORIZED, err)
			return
		}

		res, err = h.authSvc.RefreshToken(context.TODO(), claim)
		if err != nil {
			protocol.SendErrMsg(requestedTime, err)
			return
		}
	}

	protocol.SendSuccessMsg(requestedTime, res)
}

func (h DeribitHandler) DeribitParseBuy(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, userID)
	protocol.UpgradeProtocol(userID, ID)

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
	})
	if err != nil {
		if validation != nil {
			protocol.SendValidationMsg(ID, *validation, err)
			return
		}

		protocol.SendErrMsg(ID, err)
		return
	}

	protocol.SendSuccessMsg(userID, order)
}

func (h DeribitHandler) DeribitParseSell(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, userID)
	protocol.UpgradeProtocol(userID, ID)

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
	})
	if err != nil {
		if validation != nil {
			protocol.SendValidationMsg(ID, *validation, err)
			return
		}

		protocol.SendErrMsg(ID, err)
		return
	}

	protocol.SendSuccessMsg(userID, order)
}

func (h DeribitHandler) DeribitParseEdit(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, userID)
	protocol.UpgradeProtocol(userID, ID)

	// Call service
	order, err := h.svc.DeribitParseEdit(r.Request.Context(), userID, deribitModel.DeribitEditRequest{
		Id:      msg.Params.Id,
		Price:   msg.Params.Price,
		Amount:  msg.Params.Amount,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(ID, err)
		return
	}

	protocol.SendSuccessMsg(userID, order)
}

func (h DeribitHandler) DeribitParseCancel(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, userID)
	protocol.UpgradeProtocol(userID, ID)

	// Call service
	order, err := h.svc.DeribitParseCancel(r.Request.Context(), userID, deribitModel.DeribitCancelRequest{
		Id:      msg.Params.Id,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(ID, err)
		return
	}

	protocol.SendSuccessMsg(userID, order)
}

func (h DeribitHandler) DeribitCancelByInstrument(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, userID)
	protocol.UpgradeProtocol(userID, ID)

	// Call service
	order, err := h.svc.DeribitCancelByInstrument(r.Request.Context(), userID, deribitModel.DeribitCancelByInstrumentRequest{
		InstrumentName: msg.Params.InstrumentName,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(ID, err)
		return
	}

	protocol.SendSuccessMsg(userID, order)
}

func (h DeribitHandler) DeribitCancelAll(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, userID)
	protocol.UpgradeProtocol(userID, ID)

	// Call service
	order, err := h.svc.DeribitParseCancelAll(r.Request.Context(), userID, deribitModel.DeribitCancelAllRequest{
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(ID, err)
		return
	}

	protocol.SendSuccessMsg(userID, order)
}

func (h DeribitHandler) PrivateGetHandler(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	switch r.Param("method") {
	case "get_user_trades_by_instrument":
		msg := &deribitModel.RequestDto[deribitModel.GetUserTradesByInstrumentParams]{}

		if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
			protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
			return
		}

		// Number of requested items, default - 10
		if msg.Params.Count <= 0 {
			msg.Params.Count = 10
		}

		claim, err := authService.ClaimJWT(msg.Params.AccessToken)
		if err != nil {
			protocol.SendValidationMsg(userID, validation_reason.UNAUTHORIZED, err)
			return
		}

		ID := utils.GetKeyFromIdUserID(msg.Id, claim.UserID)
		protocol.UpgradeProtocol(userID, ID)

		res := h.svc.DeribitGetUserTradesByInstrument(
			r.Request.Context(),
			claim.UserID,
			deribitModel.DeribitGetUserTradesByInstrumentsRequest{
				InstrumentName: msg.Params.InstrumentName,
				Count:          msg.Params.Count,
				StartTimestamp: msg.Params.StartTimestamp,
				EndTimestamp:   msg.Params.EndTimestamp,
				Sorting:        msg.Params.Sorting,
			},
		)
		protocol.SendSuccessMsg(userID, res)
		break
	case "get_open_orders_by_instrument":
		msg := &deribitModel.RequestDto[deribitModel.GetOpenOrdersByInstrumentParams]{}
		if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
			protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
			return
		}

		// type parameter
		if msg.Params.Type == "" {
			msg.Params.Type = "all"
		}

		claim, err := authService.ClaimJWT(msg.Params.AccessToken)
		if err != nil {
			protocol.SendValidationMsg(userID, validation_reason.UNAUTHORIZED, err)
			return
		}

		ID := utils.GetKeyFromIdUserID(msg.Id, claim.UserID)
		protocol.UpgradeProtocol(userID, ID)

		res := h.svc.DeribitGetOpenOrdersByInstrument(
			context.TODO(),
			claim.UserID,
			deribitModel.DeribitGetOpenOrdersByInstrumentRequest{
				InstrumentName: msg.Params.InstrumentName,
				Type:           msg.Params.Type,
			},
		)

		protocol.SendSuccessMsg(ID, res)
		break
	case "get_order_history_by_instrument":
		msg := &deribitModel.RequestDto[deribitModel.GetOrderHistoryByInstrumentParams]{}
		if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
			protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
			return
		}

		// parameter default value
		if msg.Params.Count <= 0 {
			msg.Params.Count = 20
		}

		claim, err := authService.ClaimJWT(msg.Params.AccessToken)
		if err != nil {
			protocol.SendValidationMsg(userID, validation_reason.UNAUTHORIZED, err)
			return
		}

		ID := utils.GetKeyFromIdUserID(msg.Id, claim.UserID)
		protocol.UpgradeProtocol(userID, ID)

		res := h.svc.DeribitGetGetOrderHistoryByInstrument(
			context.TODO(),
			claim.UserID,
			deribitModel.DeribitGetOrderHistoryByInstrumentRequest{
				InstrumentName:  msg.Params.InstrumentName,
				Count:           msg.Params.Count,
				Offset:          msg.Params.Offset,
				IncludeOld:      msg.Params.IncludeOld,
				IncludeUnfilled: msg.Params.IncludeUnfilled,
			},
		)

		protocol.SendSuccessMsg(ID, res)
		break
	default:
		protocol.SendValidationMsg(userID, validation_reason.NONE, errors.New("invalid method"))
		break
	}
}

func (h DeribitHandler) PublicGetHandler(r *gin.Context) {
	requestedTime := uint64(time.Now().UnixMicro())

	if ok := protocol.RegisterProtocolRequest(requestedTime, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(requestedTime, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	switch r.Param("method") {
	case "get_instruments":
		msg := &deribitModel.RequestDto[deribitModel.GetInstrumentsParams]{}
		if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
			protocol.SendValidationMsg(requestedTime, validation_reason.PARSE_ERROR, err)
			return
		}

		result := h.svc.DeribitGetInstruments(context.TODO(), deribitModel.DeribitGetInstrumentsRequest{
			Currency: msg.Params.Currency,
			Expired:  msg.Params.Expired,
		})

		protocol.SendSuccessMsg(requestedTime, result)
		break
	case "get_order_book":

		msg := &deribitModel.RequestDto[deribitModel.GetOrderBookParams]{}
		if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
			protocol.SendValidationMsg(requestedTime, validation_reason.PARSE_ERROR, err)
			return
		}

		result := h.svc.GetOrderBook(context.TODO(), deribitModel.DeribitGetOrderBookRequest{
			InstrumentName: msg.Params.InstrumentName,
			Depth:          msg.Params.Depth,
		})

		protocol.SendSuccessMsg(requestedTime, result)
		break
	case "test":
		protocol.SendSuccessMsg(requestedTime, gin.H{
			"version": "1.2.26",
		})
		break
	default:
		protocol.SendValidationMsg(requestedTime, validation_reason.NONE, errors.New("invalid method"))
		break
	}
}
