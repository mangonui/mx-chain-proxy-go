package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/multiversx/mx-chain-proxy-go/api/middleware"
	"github.com/multiversx/mx-chain-proxy-go/config"
	"github.com/multiversx/mx-chain-proxy-go/data"
	"github.com/stretchr/testify/require"
)

type versionsRegistryStub struct {
	versions map[string]*data.VersionData
	err      error
}

func (vrs *versionsRegistryStub) AddVersion(version string, versionData *data.VersionData) error {
	if vrs.versions == nil {
		vrs.versions = make(map[string]*data.VersionData)
	}
	vrs.versions[version] = versionData
	return nil
}

func (vrs *versionsRegistryStub) GetAllVersions() (map[string]*data.VersionData, error) {
	if vrs.err != nil {
		return nil, vrs.err
	}
	if vrs.versions == nil {
		return map[string]*data.VersionData{}, nil
	}
	return vrs.versions, nil
}

func (vrs *versionsRegistryStub) IsInterfaceNil() bool {
	return vrs == nil
}

type statusMetricsExtractorStub struct{}

func (smes *statusMetricsExtractorStub) AddRequestData(_ string, _ bool, _ time.Duration) {
}

func (smes *statusMetricsExtractorStub) IsInterfaceNil() bool {
	return smes == nil
}

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

func createTestServer(t *testing.T, corsConfig config.CorsConfig) *http.Server {
	t.Helper()

	server, err := CreateServer(
		&versionsRegistryStub{},
		8080,
		config.ApiLoggingConfig{},
		config.CredentialsConfig{},
		&statusMetricsExtractorStub{},
		1,
		false,
		false,
		corsConfig,
	)
	require.NoError(t, err)
	require.NotNil(t, server)
	require.NotNil(t, server.Handler)

	return server
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

func TestCreateServerWithEmptyCorsConfigDoesNotPanic(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() {
		server := createTestServer(t, config.CorsConfig{})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/missing", nil)
		request.Header.Set("Origin", "https://dapp.example.com")

		server.Handler.ServeHTTP(recorder, request)

		require.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestCreateServerWithExplicitEmptyAllowedOriginsDoesNotPanic(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() {
		server := createTestServer(t, config.CorsConfig{
			AllowedOrigins: []string{},
		})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodOptions, "/missing", nil)
		request.Header.Set("Origin", "https://dapp.example.com")
		request.Header.Set("Access-Control-Request-Method", http.MethodGet)

		server.Handler.ServeHTTP(recorder, request)

		require.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestCreateServerWithConfiguredAllowedOriginAppliesCorsMiddleware(t *testing.T) {
	t.Parallel()

	server := createTestServer(t, config.CorsConfig{
		AllowedOrigins: []string{"https://dapp.example.com"},
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Origin", "Content-Type", "Accept"},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/missing", nil)
	request.Header.Set("Origin", "https://dapp.example.com")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)

	server.Handler.ServeHTTP(recorder, request)

	require.Equal(t, "https://dapp.example.com", recorder.Header().Get("Access-Control-Allow-Origin"))
	require.True(t, strings.Contains(recorder.Header().Get("Access-Control-Allow-Methods"), http.MethodGet))
	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestBuildCorsConfigDefaultsMethodsHeadersAndMaxAge(t *testing.T) {
	t.Parallel()

	corsConfig := buildCorsConfig(config.CorsConfig{
		AllowedOrigins: []string{"https://dapp.example.com"},
		MaxAgeSeconds:  3600,
	})

	require.Equal(t, []string{http.MethodGet, http.MethodPost, http.MethodHead, http.MethodOptions}, corsConfig.AllowMethods)
	require.Equal(t, []string{"Origin", "Content-Type", "Accept", "Authorization"}, corsConfig.AllowHeaders)
	require.Equal(t, time.Hour, corsConfig.MaxAge)
}

func TestCreateServerRejectsNilStatusMetricsExtractorBeforeUsingCorsHandler(t *testing.T) {
	t.Parallel()

	_, err := CreateServer(
		&versionsRegistryStub{},
		8080,
		config.ApiLoggingConfig{},
		config.CredentialsConfig{},
		nil,
		1,
		false,
		false,
		config.CorsConfig{},
	)

	require.Equal(t, middleware.ErrNilStatusMetricsExtractor, err)
}
