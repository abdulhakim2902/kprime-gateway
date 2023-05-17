package helpers

import (
	"gateway/pkg/ws"

	"git.devucc.name/dependencies/utilities/types/validation_reason"
)

func SendValidationResponse(
	c *ws.Client,
	validation validation_reason.ValidationReason,
	msgID, reqTime uint64,
	userId, reason *string,
) {

	code, msg := validation.Code()

	validationReason := validation.String()
	if reason != nil {
		validationReason = *reason
	}

	c.SendErrorMessage(ws.WebsocketResponseErrMessage{
		Params: ws.SendMessageParams{
			ID:            msgID,
			RequestedTime: reqTime,
			UserID:        *userId,
		},

		Message: msg,
		Code:    code,
		Data: ws.ReasonMessage{
			Reason: validationReason,
		},
	})
}
