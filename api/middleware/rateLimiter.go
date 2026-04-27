package middleware

import (
	"container/list"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/multiversx/mx-chain-proxy-go/data"
)

// ReturnCodeRequestError defines a request which hasn't been executed successfully due to a bad request received
const ReturnCodeRequestError string = "bad_request"

const defaultMaxTrackedKeysPerWindow = 100000

type rateLimiterEntry struct {
	requests uint64
	element  *list.Element
}

type rateLimiter struct {
	requestsMap    map[string]*rateLimiterEntry
	admissionOrder *list.List
	mutRequestsMap sync.RWMutex
	limits         map[string]uint64
	countDuration  time.Duration
	maxTrackedKeys int
}

// NewRateLimiter returns a new instance of rateLimiter
func NewRateLimiter(limits map[string]uint64, countDuration time.Duration) (*rateLimiter, error) {
	if limits == nil {
		return nil, ErrNilLimitsMapForEndpoints
	}
	return &rateLimiter{
		requestsMap:    make(map[string]*rateLimiterEntry),
		admissionOrder: list.New(),
		limits:         limits,
		countDuration:  countDuration,
		maxTrackedKeys: defaultMaxTrackedKeysPerWindow,
	}, nil
}

// MiddlewareHandlerFunc returns the gin middleware for limiting the number of requests for a given endpoint
func (rl *rateLimiter) MiddlewareHandlerFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		endpoint := c.FullPath()

		limitForEndpoint, isEndpointLimited := rl.limits[endpoint]
		if !isEndpointLimited {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		key := fmt.Sprintf("%s_%s", endpoint, clientIP)

		numRequests := rl.addInRequestsMap(key)
		if numRequests >= limitForEndpoint {
			printMessage := fmt.Sprintf("your IP exceeded the limit of %d requests in %v for this endpoint", limitForEndpoint, rl.countDuration)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, data.GenericAPIResponse{
				Data:  nil,
				Error: printMessage,
				Code:  data.ReturnCode(ReturnCodeRequestError),
			})
			return
		}

		c.Next()
	}
}

func (rl *rateLimiter) addInRequestsMap(key string) uint64 {
	rl.mutRequestsMap.Lock()
	defer rl.mutRequestsMap.Unlock()

	entry, ok := rl.requestsMap[key]
	if ok {
		entry.requests++
		rl.admissionOrder.MoveToBack(entry.element)
		return entry.requests
	}

	if len(rl.requestsMap) >= rl.maxTrackedKeys {
		rl.evictOldestTrackedKey()
	}

	element := rl.admissionOrder.PushBack(key)
	rl.requestsMap[key] = &rateLimiterEntry{
		requests: 1,
		element:  element,
	}

	return 1
}

func (rl *rateLimiter) evictOldestTrackedKey() {
	oldest := rl.admissionOrder.Front()
	if oldest == nil {
		return
	}

	key, ok := oldest.Value.(string)
	if !ok {
		rl.admissionOrder.Remove(oldest)
		return
	}

	delete(rl.requestsMap, key)
	rl.admissionOrder.Remove(oldest)
}

// ResetMap has to be called from outside at a given interval so the requests map will be cleaned and older restrictions
// would be erased
func (rl *rateLimiter) ResetMap(version string) {
	rl.mutRequestsMap.Lock()
	rl.requestsMap = make(map[string]*rateLimiterEntry)
	rl.admissionOrder = list.New()
	rl.mutRequestsMap.Unlock()

	log.Info("rate limiter map has been reset", "version", version, "time", time.Now())
}

// IsInterfaceNil returns true if there is no value under the interface
func (rl *rateLimiter) IsInterfaceNil() bool {
	return rl == nil
}
