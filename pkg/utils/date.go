package utils

import "time"

func MakeTimestamp(date time.Time) int64 {
	return date.UnixNano() / int64(time.Millisecond)
}
