package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestSizeLimiterRejectsKnownOversizedBody(t *testing.T) {
	t.Parallel()

	engine := gin.New()
	engine.Use(RequestSizeLimiter(4))
	engine.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("12345"))
	resp := httptest.NewRecorder()

	engine.ServeHTTP(resp, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, resp.Code)
}

func TestRequestSizeLimiterWrapsUnknownLengthBody(t *testing.T) {
	t.Parallel()

	engine := gin.New()
	engine.Use(RequestSizeLimiter(4))
	engine.POST("/test", func(c *gin.Context) {
		_, err := c.GetRawData()
		require.Error(t, err)
		c.Status(http.StatusBadRequest)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("12345"))
	req.ContentLength = -1
	resp := httptest.NewRecorder()

	engine.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
}
