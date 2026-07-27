package urlutil

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// ParseInt64Array reads a repeated query parameter and returns its values as []int64.
// Values that cannot be parsed as int64 are silently skipped.
func ParseInt64Array(c *gin.Context, key string) []int64 {
	values := c.QueryArray(key)
	if len(values) == 0 {
		return nil
	}
	result := make([]int64, 0, len(values))
	for _, v := range values {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			result = append(result, n)
		}
	}
	return result
}
