package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	logger "github.com/multiversx/mx-chain-logger-go"
)

var log = logger.GetOrCreate("api/middleware")

const (
	prefixDurationTooLong      = "[too long]"
	prefixBadRequest           = "[bad request]"
	prefixInternalError        = "[internal error]"
	maxLengthRequestOrResponse = 400
	maxLoggedBodyReadSize      = 4096
)

// TODO: remove this file and use the same middleware from mx-chain-go after it is merged

type responseLoggerMiddleware struct {
	thresholdDurationForLoggingRequest time.Duration
	printRequestFunc                   func(title string, path string, duration time.Duration, status int, clientIP string, request string, response string)
}

// NewResponseLoggerMiddleware returns a new instance of responseLoggerMiddleware
func NewResponseLoggerMiddleware(thresholdDurationForLoggingRequest time.Duration) *responseLoggerMiddleware {
	rlm := &responseLoggerMiddleware{
		thresholdDurationForLoggingRequest: thresholdDurationForLoggingRequest,
	}

	rlm.printRequestFunc = rlm.printRequest

	return rlm
}

// MiddlewareHandlerFunc logs detail about a request if it is not successful or it's duration is higher than a threshold
func (rlm *responseLoggerMiddleware) MiddlewareHandlerFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		t := time.Now()

		// read the body for logging purposes and restore it into the context
		var bodyBytes []byte
		requestBodyString := ""
		shouldReadRequestBody := c.Request.Body != nil && c.Request.ContentLength >= 0 && c.Request.ContentLength <= maxLoggedBodyReadSize
		if shouldReadRequestBody {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			requestBodyString = redactSensitiveFields(string(bodyBytes))
		} else if c.Request.Body != nil {
			requestBodyString = "[request body omitted]"
		}

		bw := &bodyWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = bw

		c.Next()

		latency := time.Since(t)
		status := c.Writer.Status()

		shouldLogRequest := latency > rlm.thresholdDurationForLoggingRequest || c.Writer.Status() != http.StatusOK
		if shouldLogRequest {
			requestBodyString = prepareLog(requestBodyString)
			responseBodyString := prepareLog(redactSensitiveFields(bw.body.String()))
			rlm.logRequestAndResponse(c, latency, status, requestBodyString, responseBodyString)
		}
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (rlm *responseLoggerMiddleware) IsInterfaceNil() bool {
	return rlm == nil
}

func (rlm *responseLoggerMiddleware) logRequestAndResponse(c *gin.Context, duration time.Duration, status int, request string, response string) {
	title := rlm.computeLogTitle(status)

	rlm.printRequestFunc(title, c.Request.RequestURI, duration, status, c.ClientIP(), request, response)
}

func (rlm *responseLoggerMiddleware) computeLogTitle(status int) string {
	logPrefix := prefixDurationTooLong
	if status == http.StatusBadRequest {
		logPrefix = prefixBadRequest
	} else if status == http.StatusInternalServerError {
		logPrefix = prefixInternalError
	} else if status != http.StatusOK {
		logPrefix = fmt.Sprintf("http code %d", status)
	}

	return fmt.Sprintf("%s api request", logPrefix)
}

func (rlm *responseLoggerMiddleware) printRequest(title string, path string, duration time.Duration, status int, clientIP string, request string, response string) {
	log.Warn(title,
		"path", path,
		"duration", duration,
		"status", status,
		"client IP", clientIP,
		"request", request,
		"response", response,
	)
}

func prepareLog(str string) string {
	truncated := false
	if len(str) > maxLengthRequestOrResponse {
		str = str[:maxLengthRequestOrResponse]
		truncated = true
	}

	var b strings.Builder
	b.Grow(len(str))
	for _, ch := range str {
		if !unicode.IsSpace(ch) {
			b.WriteRune(ch)
		}
	}

	result := b.String()
	if truncated {
		return result + "..."
	}
	return result
}

func redactSensitiveFields(str string) string {
	sensitiveFields := []string{"password", "passphrase", "mnemonic", "secretKey", "privateKey", "pem"}
	redacted := str
	for _, field := range sensitiveFields {
		redacted = redactField(redacted, field)
	}

	return redacted
}

func redactField(str string, field string) string {
	lower := strings.ToLower(str)
	lowerField := strings.ToLower(field)
	for {
		idx := strings.Index(lower, `"`+lowerField+`"`)
		if idx < 0 {
			return str
		}
		colon := strings.Index(str[idx:], ":")
		if colon < 0 {
			return str
		}
		valueStart := idx + colon + 1
		for valueStart < len(str) && unicode.IsSpace(rune(str[valueStart])) {
			valueStart++
		}
		if valueStart >= len(str) || str[valueStart] != '"' {
			return str
		}
		valueEnd := valueStart + 1
		for valueEnd < len(str) {
			if str[valueEnd] == '"' && str[valueEnd-1] != '\\' {
				break
			}
			valueEnd++
		}
		if valueEnd >= len(str) {
			return str
		}
		if str[valueStart:valueEnd+1] == `"[REDACTED]"` {
			return str
		}
		str = str[:valueStart] + `"[REDACTED]"` + str[valueEnd+1:]
		lower = strings.ToLower(str)
	}
}

type bodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyWriter) Write(b []byte) (int, error) {
	if w.body.Len() < maxLoggedBodyReadSize {
		remaining := maxLoggedBodyReadSize - w.body.Len()
		if len(b) > remaining {
			_, _ = w.body.Write(b[:remaining])
		} else {
			_, _ = w.body.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}
