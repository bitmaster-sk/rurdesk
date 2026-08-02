package urlutil

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ParseInt64Array reads an int64 list query parameter. Accepts both wire forms
// and their combination:
//
//	?k=1&k=2    repeated (canonical; MCP tools)
//	?k=1,2      comma-joined (Angular client)
//	?k=1,2&k=3  mixed
//
// Elements are trimmed; empty and unparseable ones are skipped ("1,,2" is two
// ids). Returns nil when nothing usable was found — callers gate on len() > 0.
func ParseInt64Array(c *gin.Context, key string) []int64 {
	values := c.QueryArray(key)
	if len(values) == 0 {
		return nil
	}
	result := make([]int64, 0, len(values))
	for _, value := range values {
		for _, element := range strings.Split(value, ",") {
			element = strings.TrimSpace(element)
			if element == "" {
				continue
			}
			if parsed, err := strconv.ParseInt(element, 10, 64); err == nil {
				result = append(result, parsed)
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
