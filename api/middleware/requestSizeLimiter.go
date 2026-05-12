package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/multiversx/mx-chain-proxy-go/data"
)

const DefaultMaxRequestBodySize = 4 << 20

// RequestSizeLimiter wraps request bodies before handlers bind JSON.
func RequestSizeLimiter(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil {
			c.Next()
			return
		}
		if maxBytes <= 0 {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, data.GenericAPIResponse{
				Data:  nil,
				Error: "request body too large",
				Code:  data.ReturnCode(ReturnCodeRequestError),
			})
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
