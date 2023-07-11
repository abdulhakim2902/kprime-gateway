package utils

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"github.com/Undercurrent-Technologies/kprime-utilities/types"
)

func GetKeyFromIdUserID(id uint64, userID string) string {
	return strconv.FormatUint(id, 10) + "-" + userID
}

func GetIdUserIDFromKey(key string) (id uint64, userID string) {
	splitted := strings.Split(key, "-")

	var err error
	id, err = strconv.ParseUint(splitted[0], 10, 0)
	if err != nil {
		return
	}

	if len(splitted) > 1 {
		userID = splitted[1]
	}
	return
}

func ArrContains(arr []string, value string) bool {
	for _, v := range arr {
		if v == value {
			return true
		}
	}
	return false
}

type Instruments struct {
	Underlying, ExpDate string
	Contracts           types.Contracts
	Strike              float64
}

func ParseInstruments(str string) (*Instruments, error) {
	substring := strings.Split(str, "-")
	if len(substring) != 4 {
		return nil, errors.New("invalid instruments")
	}

	_underlying := substring[0]
	_expDate := strings.ToUpper(substring[1])

	strike, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		return nil, errors.New("invalid instruments")
	}

	var _contracts types.Contracts
	if substring[3] == "P" {
		_contracts = types.PUT
	} else {
		_contracts = types.CALL
	}

	return &Instruments{_underlying, _expDate, _contracts, strike}, nil
}

func ConvertToFloat(str string) (number float64, isSuccess bool) {
	conversion, err := strconv.ParseFloat(str, 32)
	if err != nil {
		logs.Log.Err(err).Msg(fmt.Sprintf("String Conversion to Float64 Failed! %s", str))
		number = 0
		isSuccess = false
		return
	}

	// Success
	number = conversion
	isSuccess = true
	return
}
