package utils

import (
	"encoding/json"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	validator "github.com/go-playground/validator/v10"
)

func UnmarshalAndValidate[T any](r *gin.Context, data *T) (err error) {
	if r.Request.Method == "POST" {
		err = r.ShouldBindBodyWith(data, binding.JSON)
	} else {
		err = r.ShouldBindQuery(data)
	}

	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		return
	}

	validate := validator.New()
	if err = validate.Struct(*data); err != nil {
		logs.Log.Error().Err(err).Msg("")
		return
	}

	return
}

func UnmarshalAndValidateWS[T any](input interface{}, data *T) error {
	bytes, err := json.Marshal(input)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		return err
	}

	if err := json.Unmarshal(bytes, data); err != nil {
		logs.Log.Error().Err(err).Msg("")
		return err
	}

	validate := validator.New()
	if err = validate.Struct(*data); err != nil {
		logs.Log.Error().Err(err).Msg("")
		return err
	}

	return nil
}
