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

func MDBFindCredentialByKey(key string) (credential schema.UserCredential, reason validation_reason.ValidationReason, err error) {
	reason = validation_reason.NONE

	var result interface{}
	result, err = Schemas.UserCredential.FindOne("key", key)
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

	credentialCast, ok := result.(schema.UserCredential)
	if !ok {
		reason = validation_reason.UNAUTHORIZED
		err = errors.New(reason.String())
		return
	}

	credential = credentialCast
	return
}

func MDBFindUserAndCredentialWithKey(key string) (
	user schema.User,
	credential schema.UserCredential,
	reason *validation_reason.ValidationReason,
) {
	var err error
	var rsn validation_reason.ValidationReason
	credential, rsn, err = MDBFindCredentialByKey(key)
	if err != nil {
		errReason := validation_reason.OTHER
		reason = &errReason
		if rsn != validation_reason.NONE {
			reason = &rsn
		}

		logs.Log.Error().Err(err).Msg(reason.String())
		return
	}

	user, rsn, err = MDBFindUserById(credential.UserID)
	if err != nil {
		errReason := validation_reason.OTHER
		reason = &errReason
		if rsn != validation_reason.NONE {
			reason = &rsn
		}

		logs.Log.Error().Err(err).Msg(reason.String())
		return
	}

	return
}
