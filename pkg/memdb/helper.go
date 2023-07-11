package memdb

import (
	"errors"

	"gateway/schema"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"github.com/Undercurrent-Technologies/kprime-utilities/types/validation_reason"
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
