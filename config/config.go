package config

import (
	"github.com/multiversx/mx-chain-proxy-go/data"
)

// GeneralSettingsConfig will hold the general settings for a node
type GeneralSettingsConfig struct {
	ServerPort                               int
	RequestTimeoutSec                        int
	HeartbeatCacheValidityDurationSec        int
	ValStatsCacheValidityDurationSec         int
	EconomicsMetricsCacheValidityDurationSec int
	BlockCacheDurationSec                    int
	FaucetValue                              string
	RateLimitWindowDurationSeconds           int
	BalancedObservers                        bool
	BalancedFullHistoryNodes                 bool
	AllowEntireTxPoolFetch                   bool
	NumShardsTimeoutInSec                    int
	TimeBetweenNodesRequestsInSec            int
}

// Config will hold the whole config file's data
type Config struct {
	GeneralSettings        GeneralSettingsConfig
	AddressPubkeyConverter PubkeyConfig
	Marshalizer            TypeConfig
	Hasher                 TypeConfig
	ApiLogging             ApiLoggingConfig
	Cors                   CorsConfig
	Observers              []*data.NodeData
	FullHistoryNodes       []*data.NodeData
}

// CorsConfig configures the HTTP-layer Cross-Origin Resource Sharing
// policy applied by api.CreateServer.
//
// Default behaviour (when AllowedOrigins is empty): cross-origin
// requests are blocked. Operators who want backward-compatible
// permissive behaviour must opt in by setting AllowedOrigins = ["*"]
// in config.toml. This reverses the prior default of cors.Default()
// (which silently allowed all origins with credentials enabled — a
// real cross-origin info-leak surface for any browser-driven attacker
// who could reach the proxy port).
//
// AllowCredentials defaults to false; only set true if explicit
// authenticated cross-origin use cases require it.
type CorsConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAgeSeconds    int
}

// TypeConfig will map the string type configuration
type TypeConfig struct {
	Type string
}

// PubkeyConfig will map the public key configuration
type PubkeyConfig struct {
	Length          int
	Type            string
	SignatureLength int
}

// ApiLoggingConfig holds the configuration related to API requests logging
type ApiLoggingConfig struct {
	LoggingEnabled          bool
	ThresholdInMicroSeconds int
}

// CredentialsConfig holds the credential pairs
type CredentialsConfig struct {
	Credentials []data.Credential
	Hasher      TypeConfig
}
