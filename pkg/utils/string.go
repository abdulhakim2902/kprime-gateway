package utils

import (
	"fmt"
	"strconv"
)

func GetKeyFromIdUserID(id uint64, userID string) string {
	return strconv.FormatUint(id, 10) + "-" + userID
}

func ArrContains(arr []string, value string) bool {
	fmt.Println("arrContains")
	for _, v := range arr {
		fmt.Println(v, value)
		if v == value {
			return true
		}
	}
	return false
}
