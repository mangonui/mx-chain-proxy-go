package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/multiversx/mx-chain-core-go/core/check"
	"github.com/multiversx/mx-chain-proxy-go/api/groups"
	"github.com/multiversx/mx-chain-proxy-go/api/mock"
	"github.com/multiversx/mx-chain-proxy-go/common"
	"github.com/multiversx/mx-chain-proxy-go/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRateLimiter_NilLimitsMapShouldErr(t *testing.T) {
	t.Parallel()

	rl, err := NewRateLimiter(nil, time.Millisecond)
	require.Equal(t, ErrNilLimitsMapForEndpoints, err)
	require.True(t, check.IfNil(rl))
}

func TestNewRateLimiter_ShouldWork(t *testing.T) {
	t.Parallel()

	rl, err := NewRateLimiter(map[string]uint64{"abc": 5}, time.Millisecond)
	require.NoError(t, err)
	require.False(t, check.IfNil(rl))
}

func TestRateLimiter_IpRestrictionRaisedAndErased(t *testing.T) {
	t.Parallel()

	rl, err := NewRateLimiter(map[string]uint64{"/address/:address": 2}, time.Millisecond)
	require.NoError(t, err)

	facade := &mock.FacadeStub{
		GetAccountHandler: func(address string, _ common.AccountQueryOptions) (*data.AccountModel, error) {
			return &data.AccountModel{
				Account: data.Account{
					Address: address,
					Nonce:   1,
					Balance: "100",
				},
			}, nil
		},
	}
	addressGroup, err := groups.NewAccountsGroup(facade)
	require.NoError(t, err)
	ws := startProxyServer(addressGroup, rl, 2, "/address")

	resp := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(resp)
	req, _ := http.NewRequestWithContext(context, "GET", "/address/test", nil)
	ws.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	req, _ = http.NewRequestWithContext(context, "GET", "/address/test", nil)
	resp = httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusTooManyRequests, resp.Code)

	req, _ = http.NewRequestWithContext(context, "GET", "/address/test", nil)
	resp = httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusTooManyRequests, resp.Code)

	rl.ResetMap("")

	req, _ = http.NewRequestWithContext(context, "GET", "/address/test", nil)
	resp = httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestRateLimiter_EndpointNotLimitedShouldNotRaiseRestrictions(t *testing.T) {
	t.Parallel()

	rl, err := NewRateLimiter(map[string]uint64{"/address/:address/nonce": 1}, time.Millisecond)
	require.NoError(t, err)

	facade := &mock.FacadeStub{
		GetAccountHandler: func(address string, _ common.AccountQueryOptions) (*data.AccountModel, error) {
			return &data.AccountModel{
				Account: data.Account{
					Address: address,
					Nonce:   1,
					Balance: "100",
				},
			}, nil
		},
	}
	addressGroup, err := groups.NewAccountsGroup(facade)
	require.NoError(t, err)
	ws := startProxyServer(addressGroup, rl, 1, "/address")

	resp := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(resp)
	req, _ := http.NewRequestWithContext(context, "GET", "/address/test", nil)
	ws.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	req, _ = http.NewRequestWithContext(context, "GET", "/address/test", nil)
	resp = httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	req, _ = http.NewRequestWithContext(context, "GET", "/address/test", nil)
	resp = httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestRateLimiter_EndpointNotLimitedStillCallsNextHandler(t *testing.T) {
	t.Parallel()

	rl, err := NewRateLimiter(map[string]uint64{"/limited": 1}, time.Millisecond)
	require.NoError(t, err)

	engine := gin.New()
	var called atomic.Bool
	engine.Use(rl.MiddlewareHandlerFunc())
	engine.GET("/open", func(c *gin.Context) {
		called.Store(true)
		c.Status(http.StatusNoContent)
	})

	req, err := http.NewRequest(http.MethodGet, "/open", nil)
	require.NoError(t, err)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	require.True(t, called.Load())
	require.Equal(t, http.StatusNoContent, resp.Code)
}

func TestRateLimiter_TrackedKeyCountIsBounded(t *testing.T) {
	t.Parallel()

	rl, err := NewRateLimiter(map[string]uint64{"/limited": 10}, time.Millisecond)
	require.NoError(t, err)

	rl.maxTrackedKeys = 2

	require.Equal(t, uint64(1), rl.addInRequestsMap("/limited_1.1.1.1"))
	require.Equal(t, uint64(1), rl.addInRequestsMap("/limited_2.2.2.2"))
	require.Len(t, rl.requestsMap, 2)

	require.Equal(t, uint64(1), rl.addInRequestsMap("/limited_3.3.3.3"))
	require.Len(t, rl.requestsMap, 2)
	_, oldestStillTracked := rl.requestsMap["/limited_1.1.1.1"]
	require.False(t, oldestStillTracked, "oldest tracked key should be evicted once the admission cap is reached")
}

func TestRateLimiter_ExistingTrackedKeyMovesToBackOnAccess(t *testing.T) {
	t.Parallel()

	rl, err := NewRateLimiter(map[string]uint64{"/limited": 10}, time.Millisecond)
	require.NoError(t, err)

	rl.maxTrackedKeys = 2

	require.Equal(t, uint64(1), rl.addInRequestsMap("/limited_1.1.1.1"))
	require.Equal(t, uint64(1), rl.addInRequestsMap("/limited_2.2.2.2"))
	require.Equal(t, uint64(2), rl.addInRequestsMap("/limited_1.1.1.1"))
	require.Equal(t, uint64(1), rl.addInRequestsMap("/limited_3.3.3.3"))

	_, firstKeyStillTracked := rl.requestsMap["/limited_1.1.1.1"]
	require.True(t, firstKeyStillTracked, "recently used key should remain tracked under cap pressure")
	_, secondKeyStillTracked := rl.requestsMap["/limited_2.2.2.2"]
	require.False(t, secondKeyStillTracked, "least recently used key should be evicted under cap pressure")
}

func startProxyServer(group data.GroupHandler, rateLimiter RateLimiterHandler, rateLimit uint64, path string) *gin.Engine {
	ws := gin.New()
	ws.Use(cors.Default())
	routes := ws.Group(path)
	apiConfig := data.ApiRoutesConfig{
		APIPackages: map[string]data.APIPackageConfig{
			"address": {Routes: []data.RouteConfig{
				{
					Name:      "/:address",
					Open:      true,
					Secured:   false,
					RateLimit: rateLimit,
				},
			},
			},
		},
	}
	group.RegisterRoutes(routes, apiConfig, emptyGinHandler, rateLimiter.MiddlewareHandlerFunc(), emptyGinHandler)
	return ws
}
