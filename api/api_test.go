package api

import (
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type rateLimiterHandlerStub struct {
	resetCount int32
}

func (rl *rateLimiterHandlerStub) MiddlewareHandlerFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func (rl *rateLimiterHandlerStub) ResetMap(_ string) {
	atomic.AddInt32(&rl.resetCount, 1)
}

func (rl *rateLimiterHandlerStub) IsInterfaceNil() bool {
	return rl == nil
}

func TestConstantTimeStringEquals(t *testing.T) {
	t.Parallel()

	require.True(t, constantTimeStringEquals("abcd", "abcd"))
	require.False(t, constantTimeStringEquals("abcd", "abce"))
	require.False(t, constantTimeStringEquals("abcd", "abc"))
}

func TestStartRateLimiterResetStopsAfterCancel(t *testing.T) {
	t.Parallel()

	rl := &rateLimiterHandlerStub{}
	cancel := startRateLimiterReset(1, rl, "v1")

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&rl.resetCount) >= 1
	}, 1500*time.Millisecond, 20*time.Millisecond)

	beforeCancel := atomic.LoadInt32(&rl.resetCount)
	cancel()

	time.Sleep(1100 * time.Millisecond)

	require.Equal(t, beforeCancel, atomic.LoadInt32(&rl.resetCount))
}

func TestSkValidatorRejectsMalformedSecretKeys(t *testing.T) {
	t.Parallel()

	require.False(t, skValidator(nil, reflect.ValueOf("not-hex"), reflect.Value{}, reflect.Value{}, nil, reflect.String, ""))
	require.False(t, skValidator(nil, reflect.ValueOf("abcd"), reflect.Value{}, reflect.Value{}, nil, reflect.String, ""))
}

func TestSkValidatorAcceptsValidSecretKeyHex(t *testing.T) {
	t.Parallel()

	validKey := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	require.True(t, skValidator(nil, reflect.ValueOf(validKey), reflect.Value{}, reflect.Value{}, nil, reflect.String, ""))
}
