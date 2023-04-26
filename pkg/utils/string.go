package utils

import (
	"strconv"
)

func GetKeyFromIdUserID(id uint64, userID string) string {
	return strconv.FormatUint(id, 10) + "-" + userID
}
