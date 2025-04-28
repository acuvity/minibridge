package sanitize

import "bytes"

// Data sanitizes the data for internal
// transport. It removed all trailing '\n' and `\r`
func Data(data []byte) []byte {

	return bytes.TrimRight(data, "\n\r")
}
