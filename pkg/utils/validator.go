package utils

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	validator "github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
)

var (
	validators *validator.Validate
	once       sync.Once
	Trans      ut.Translator
)

type errs struct {
	errs []string
}

func (hs errs) Error() string {
	return strings.Join(hs.errs, ", ")
}

func init() {
	once.Do(func() {
		en_locales := en.New()
		universalTranslator := ut.New(en_locales, en_locales)
		Trans, _ = universalTranslator.GetTranslator("en")
		validators = validator.New()

		if err := en_translations.RegisterDefaultTranslations(validators, Trans); err != nil {
			logs.Log.Fatal().Err(err).Msg("")
		}

		validators.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]

			if name == "-" {
				return ""
			}

			return name
		})
	})
}

func validate(i any) error {
	var er errs

	err := validators.Struct(i)
	if err == nil {
		return nil
	}

	errs := err.(validator.ValidationErrors)

	for _, v := range errs {
		translate := v.Translate(Trans)
		er.errs = append(er.errs, translate)
	}

	return er
}

func UnmarshalAndValidate[T any](r *gin.Context, data *T) (err error) {
	if r.Request.Method == "POST" {
		body, ok := r.Get("body")
		if !ok {
			err = errors.New("invalid request")
			return
		}

		err = json.Unmarshal(body.([]byte), data)
	} else {
		err = r.ShouldBindQuery(data)
	}

	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		return
	}

	if err = validate(*data); err != nil {
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

	if err = validate(*data); err != nil {
		logs.Log.Error().Err(err).Msg("")
		return err
	}

	return nil
}
