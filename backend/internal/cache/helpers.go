package cache

import "strconv"

func parseInt(b []byte) int64 {
	n, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
