package helpers

import (
	"gateway/pkg/ws"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/types/validation_reason"
)

func SendValidationResponse(
	c *ws.Client,
	validation validation_reason.ValidationReason,
	msgID, reqTime uint64,
	userId, reason *string,
) {

	code, _, msg := validation.Code()

	validationReason := validation.String()
	if reason != nil {
		validationReason = *reason
	}

	params := ws.SendMessageParams{
		ID:            msgID,
		RequestedTime: reqTime,
	}

	if userId != nil {
		params.UserID = *userId
	}

	// Catch the validation to log
	logs.Log.Debug().Str("validation_reason", validationReason).Msg(msg)

	c.SendErrorMessage(ws.WebsocketResponseErrMessage{
		Params: params,

		Message: msg,
		Code:    code,
		Data: ws.ReasonMessage{
			Reason: validationReason,
		},
	})
}
