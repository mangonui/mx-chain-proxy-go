package api

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/multiversx/mx-chain-core-go/hashing"
	"github.com/multiversx/mx-chain-core-go/hashing/factory"
	"github.com/multiversx/mx-chain-core-go/hashing/sha256"
	crypto "github.com/multiversx/mx-chain-crypto-go"
	"github.com/multiversx/mx-chain-crypto-go/signing"
	"github.com/multiversx/mx-chain-crypto-go/signing/ed25519"
	logger "github.com/multiversx/mx-chain-logger-go"
	"github.com/multiversx/mx-chain-proxy-go/api/middleware"
	"github.com/multiversx/mx-chain-proxy-go/config"
	"github.com/multiversx/mx-chain-proxy-go/data"
	"gopkg.in/go-playground/validator.v8"
)

var log = logger.GetOrCreate("api")

type validatorInput struct {
	Name      string
	Validator validator.Func
}

// CreateServer creates a HTTP server.
//
// corsConfig governs the CORS policy applied to the gin engine. An
// empty AllowedOrigins slice produces a restrictive (no cross-origin)
// policy — this is the safe default. Operators opting into
// permissive cross-origin behaviour must explicitly populate
// AllowedOrigins (e.g. ["*"] for the legacy "allow everything"
// behaviour).
func CreateServer(
	versionsRegistry data.VersionsRegistryHandler,
	port int,
	apiLoggingConfig config.ApiLoggingConfig,
	credentialsConfig config.CredentialsConfig,
	statusMetricsExtractor middleware.StatusMetricsExtractor,
	rateLimitTimeWindowInSeconds int,
	isProfileModeActivated bool,
	shouldStartSwaggerUI bool,
	corsConfig config.CorsConfig,
) (*http.Server, error) {
	ws := gin.Default()
	ws.Use(cors.New(buildCorsConfig(corsConfig)))

	err := registerValidators()
	if err != nil {
		return nil, err
	}

	resetLoopCancels, err := registerRoutes(ws, versionsRegistry, apiLoggingConfig, credentialsConfig, statusMetricsExtractor, rateLimitTimeWindowInSeconds, isProfileModeActivated, shouldStartSwaggerUI)
	if err != nil {
		return nil, err
	}

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           ws,
		ReadHeaderTimeout: 5 * time.Second,
	}
	for _, cancel := range resetLoopCancels {
		httpServer.RegisterOnShutdown(cancel)
	}

	return httpServer, nil
}

// buildCorsConfig translates the proxy's CorsConfig into a gin-contrib
// cors.Config. Defaults are restrictive: empty AllowedOrigins yields a
// no-cross-origin policy. Allowed methods default to safe-set GET/HEAD
// /OPTIONS plus POST (the proxy's main endpoints). Allowed headers
// default to the standard request headers needed for JSON.
func buildCorsConfig(cfg config.CorsConfig) cors.Config {
	c := cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     cfg.AllowedMethods,
		AllowHeaders:     cfg.AllowedHeaders,
		AllowCredentials: cfg.AllowCredentials,
	}
	if len(c.AllowMethods) == 0 {
		c.AllowMethods = []string{"GET", "POST", "HEAD", "OPTIONS"}
	}
	if len(c.AllowHeaders) == 0 {
		c.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	}
	if cfg.MaxAgeSeconds > 0 {
		c.MaxAge = time.Duration(cfg.MaxAgeSeconds) * time.Second
	}
	// Backward compatibility: an empty AllowOrigins slice in
	// gin-contrib/cors blocks all cross-origin traffic. That is the
	// intended safe default — operators must opt into permissive
	// behaviour explicitly via config.toml [Cors] AllowedOrigins.
	return c
}

func registerValidators() error {
	validators := []validatorInput{
		{Name: "skValidator", Validator: skValidator},
	}
	for _, validatorFunc := range validators {
		if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			err := v.RegisterValidation(validatorFunc.Name, validatorFunc.Validator)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func registerRoutes(
	ws *gin.Engine,
	versionsRegistry data.VersionsRegistryHandler,
	apiLoggingConfig config.ApiLoggingConfig,
	credentialsConfig config.CredentialsConfig,
	statusMetricsExtractor middleware.StatusMetricsExtractor,
	rateLimitTimeWindowInSeconds int,
	isProfileModeActivated bool,
	shouldStartSwaggerUI bool,
) ([]func(), error) {
	versionsMap, err := versionsRegistry.GetAllVersions()
	if err != nil {
		return nil, err
	}
	resetLoopCancels := make([]func(), 0)

	if shouldStartSwaggerUI {
		ws.Use(static.ServeRoot("/", "config/swagger"))
	}

	if apiLoggingConfig.LoggingEnabled {
		responseLoggerMiddleware := middleware.NewResponseLoggerMiddleware(time.Duration(apiLoggingConfig.ThresholdInMicroSeconds) * time.Microsecond)
		ws.Use(responseLoggerMiddleware.MiddlewareHandlerFunc())
	}

	// TODO: maybe add a flag when starting proxy if metrics should be exposed or not
	metricsMiddleware, err := middleware.NewMetricsMiddleware(statusMetricsExtractor)
	if err != nil {
		return nil, err
	}

	for version, versionData := range versionsMap {
		limitsMap := getLimitsMapForVersion(versionData)
		rateLimitTimeWindowDuration := time.Duration(rateLimitTimeWindowInSeconds) * time.Second
		rateLimiter, err := middleware.NewRateLimiter(limitsMap, rateLimitTimeWindowDuration)
		if err != nil {
			return nil, err
		}
		stopRateLimiterReset := startRateLimiterReset(rateLimitTimeWindowInSeconds, rateLimiter, version)
		resetLoopCancels = append(resetLoopCancels, stopRateLimiterReset)
		versionGroup := ws.Group(version)
		for path, group := range versionData.ApiHandler.GetAllGroups() {
			subGroup := versionGroup.Group(path)
			group.RegisterRoutes(
				subGroup,
				versionData.ApiConfig,
				getAuthenticationFunc(credentialsConfig),
				rateLimiter.MiddlewareHandlerFunc(),
				metricsMiddleware.MiddlewareHandlerFunc(),
			)
		}
	}

	if isProfileModeActivated {
		pprof.Register(ws)
	}

	return resetLoopCancels, nil
}

func getAuthenticationFunc(credentialsConfig config.CredentialsConfig) gin.HandlerFunc {
	if len(credentialsConfig.Credentials) == 0 {
		return func(c *gin.Context) {
			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				data.GenericAPIResponse{
					Data:  nil,
					Error: "no credentials found on server",
					Code:  data.ReturnCodeInternalError,
				},
			)
		}
	}

	var hasher hashing.Hasher
	var err error
	hasher, err = factory.NewHasher(credentialsConfig.Hasher.Type)
	if err != nil {
		log.Warn("cannot create hasher from config. Will use Sha256 as default", "error", err)
		hasher = sha256.NewSha256() // fallback in case the hasher creation failed
	}

	accounts := gin.Accounts{}
	for _, pair := range credentialsConfig.Credentials {
		accounts[pair.Username] = pair.Password
	}

	authenticationFunction := func(c *gin.Context) {
		user, pass, ok := c.Request.BasicAuth()
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, data.GenericAPIResponse{
				Data:  nil,
				Error: "this endpoint requires Basic Authentication",
				Code:  data.ReturnCodeRequestError,
			})
			return
		}

		userPassword, ok := accounts[user]
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, data.GenericAPIResponse{
				Data:  nil,
				Error: "username does not exist",
				Code:  data.ReturnCodeRequestError,
			})
			return
		}

		if !constantTimeStringEquals(userPassword, hex.EncodeToString(hasher.Compute(pass))) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, data.GenericAPIResponse{
				Data:  nil,
				Error: "invalid password",
				Code:  data.ReturnCodeRequestError,
			})
			return
		}
	}

	return authenticationFunction
}

func getLimitsMapForVersion(versionData *data.VersionData) map[string]uint64 {
	limitsMap := make(map[string]uint64)
	for packageName, packageConfig := range versionData.ApiConfig.APIPackages {
		for _, routeConfig := range packageConfig.Routes {
			if routeConfig.RateLimit > 0 {
				mapKey := fmt.Sprintf("/%s%s", packageName, routeConfig.Name)
				limitsMap[mapKey] = routeConfig.RateLimit
			}
		}
	}

	return limitsMap
}

func startRateLimiterReset(rateLimiterDuration int, rl middleware.RateLimiterHandler, version string) func() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(time.Duration(rateLimiterDuration) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				rl.ResetMap(version)
			case <-ctx.Done():
				return
			}
		}
	}()

	return cancel
}

// skValidator validates a secret key from user input for correctness
func skValidator(
	_ *validator.Validate,
	field reflect.Value,
	_ reflect.Value,
	_ reflect.Value,
	_ reflect.Type,
	kind reflect.Kind,
	_ string,
) bool {
	if kind != reflect.String {
		return false
	}

	return isValidSecretKeyHex(field.String())
}

func constantTimeStringEquals(provided string, expected string) bool {
	if len(provided) != len(expected) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func isValidSecretKeyHex(secretKey string) bool {
	keyBytes, err := hex.DecodeString(secretKey)
	if err != nil {
		return false
	}

	_, err = newSecretKeyGenerator().PrivateKeyFromByteArray(keyBytes)
	return err == nil
}

func newSecretKeyGenerator() crypto.KeyGenerator {
	return signing.NewKeyGenerator(ed25519.NewEd25519())
}
