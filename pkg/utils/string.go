package utils

import (
	"errors"
	"fmt"
	"gateway/pkg/constant"
	"strconv"
	"strings"
	"time"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/date"
	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	_instrumentTypes "github.com/Undercurrent-Technologies/kprime-utilities/config/types"
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

func isExpired(expDate string) bool {
	now := time.Now()
	loc, _ := time.LoadLocation("Singapore")
	if loc != nil {
		now = now.In(loc)
	}

	cy, cm, cd := now.Date()
	ey, em, ed := date.ParseDate(expDate)
	if ey < cy {
		return true
	}

	if ey == cy {
		if em < cm {
			return true
		}

		if em == cm {
			if ed < cd {
				return true
			}
		}
	}

	return false
}

func ParseInstruments(str string, checkExpired bool) (*Instruments, error) {
	substring := strings.Split(str, "-")
	if len(substring) != 4 {
		return nil, errors.New(constant.INVALID_INSTRUMENT)
	}

	// Validate Underlying Currencies
	_underlying := strings.ToUpper(substring[0])
	if _instrumentTypes.Underlying(_underlying).IsValidUnderlying() == false {
		return nil, errors.New(constant.UNSUPPORTED_CURRENCY)
	}

	// Validate Expiry Date, invalid/expired
	_expDate := strings.ToUpper(substring[1])

	if checkExpired {
		if isExpired(_expDate) {
			return nil, errors.New(constant.EXPIRED_INSTRUMENT)
		}
	}

	// Validate strike price
	if _, err := strconv.Atoi(substring[2]); err != nil {
		return nil, errors.New(constant.INVALID_STRIKE_PRICE)
	}
	strike, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		return nil, errors.New(constant.INVALID_STRIKE_PRICE)
	}

	// Validate strategy, only P and C allowed.
	substring[3] = strings.ToUpper(substring[3])
	var _contracts types.Contracts
	switch substring[3] {
	case "C":
		_contracts = types.CALL
	case "P":
		_contracts = types.PUT
	default:
		return nil, errors.New(constant.INVALID_INSTRUMENT_STRATEGY)
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
