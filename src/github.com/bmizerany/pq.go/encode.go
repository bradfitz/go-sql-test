package pq

import (
	"fmt"
)

func encodeParams(params []interface{}) (r []string) {
	r = make([]string, len(params))
	for i, param := range params {
		switch param.(type) {
		default:
			panic(fmt.Sprintf("unknown type for %T", param))
		case int, uint8, uint16, uint32, uint64, int8, int16, int32, int64:
			r[i] = fmt.Sprintf("%d", param)
		case string, []byte:
			r[i] = fmt.Sprintf("%s", param)
		case bool:
			r[i] = fmt.Sprintf("%t", param)
		}
	}

	return
}
