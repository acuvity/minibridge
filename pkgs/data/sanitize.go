package data

import "bytes"

// Sanitize sanitizes the data for internal
// transport. It removed all trailing '\n' and `\r`
func Sanitize(data []byte) []byte {

	return bytes.TrimRight(data, "\n\r")
}
