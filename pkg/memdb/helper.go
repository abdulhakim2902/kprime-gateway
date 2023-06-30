package memdb

import (
	"errors"

	"gateway/schema"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/types/validation_reason"
)

func MDBFindUserById(id string) (user schema.User, reason validation_reason.ValidationReason, err error) {
	reason = validation_reason.NONE

	var result interface{}
	result, err = Schemas.User.FindOne("id", id)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		reason = validation_reason.OTHER
		return
	}

	if result == nil {
		reason = validation_reason.UNAUTHORIZED
		err = errors.New(reason.String())
		return
	}

	userCast, ok := result.(schema.User)
	if !ok {
		reason = validation_reason.UNAUTHORIZED
		err = errors.New(reason.String())
		return
	}

	user = userCast
	return
}
