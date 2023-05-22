package utils

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	validator "github.com/go-playground/validator/v10"
)

func UnmarshalAndValidate[T any](r *gin.Context, data *T) (err error) {
	if err = r.ShouldBindJSON(data); err != nil {
		return
	}

	validate := validator.New()
	if err = validate.Struct(*data); err != nil {
		return
	}

	return
}

func UnmarshalAndValidateWS[T any](input interface{}, data *T) error {
	bytes, err := json.Marshal(input)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(bytes, data); err != nil {
		return err
	}

	validate := validator.New()
	if err = validate.Struct(*data); err != nil {
		return err
	}

	return nil
}
