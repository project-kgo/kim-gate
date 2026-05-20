package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	DefaultHTTPAddr         = ":8888"
	DefaultWebSocketPath    = "/hub"
	DefaultGRPCSocket       = "/tmp/kim-gate.sock"
	DefaultShutdownTimeout  = 10 * time.Second
	DefaultPingInterval     = 30 * time.Second
	DefaultPingTimeout      = 60 * time.Second
	DefaultRedisDSN         = "redis://localhost:6379/0"
	DefaultRedisRouteTTL    = 1 * time.Minute
	DefaultRedisPushChannel = "kim:gateway:push"
	pushUsersSuffix         = ":users"
	pushGroupSuffix         = ":group"
	pushBroadcastSuffix     = ":broadcast"
)

type Config struct {
	HTTPAddr        string
	WebSocketPath   string
	GRPCSocket      string
	ShutdownTimeout time.Duration
	PingInterval    time.Duration
	PingTimeout     time.Duration

	RedisDSN                  string
	RedisRouteTTL             time.Duration
	RedisPushChannel          string
	RedisPushUsersChannel     string
	RedisPushGroupChannel     string
	RedisPushBroadcastChannel string

	JWTSecret     string
	JWTExpiration time.Duration
}

func Load(args []string) (Config, error) {
	v := viper.New()
	setDefaults(v)
	bindEnv(v)

	fs := pflag.NewFlagSet("kim-gate", pflag.ContinueOnError)
	fs.String("config", "", "config file path")
	fs.String("http-addr", "", "hertz listen address")
	fs.String("ws-path", "", "websocket path")
	fs.String("grpc-socket", "", "grpc unix domain socket path")
	fs.Duration("shutdown-timeout", 0, "graceful shutdown timeout")
	fs.Duration("ping-interval", 0, "signalg ping interval")
	fs.Duration("ping-timeout", 0, "signalg ping timeout")
	fs.String("redis-dsn", "", "redis connection dsn")
	fs.Duration("redis-route-ttl", 0, "redis user route ttl")
	fs.String("redis-push-channel", "", "redis push pub/sub channel prefix")
	fs.String("redis-push-users-channel", "", "redis users push pub/sub channel")
	fs.String("redis-push-group-channel", "", "redis group push pub/sub channel")
	fs.String("redis-push-broadcast-channel", "", "redis broadcast push pub/sub channel")
	fs.String("jwt-secret", "", "jwt hmac secret key")
	fs.Duration("jwt-expiration", 0, "max jwt token lifetime")
	if err := fs.Parse(normalizeFlagArgs(args)); err != nil {
		return Config{}, err
	}
	if err := bindFlags(v, fs); err != nil {
		return Config{}, err
	}
	if err := readConfigFile(v, fs.Lookup("config").Value.String()); err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:                  v.GetString("http.addr"),
		WebSocketPath:             v.GetString("websocket.path"),
		GRPCSocket:                v.GetString("grpc.socket"),
		ShutdownTimeout:           v.GetDuration("shutdown.timeout"),
		PingInterval:              v.GetDuration("signalg.ping_interval"),
		PingTimeout:               v.GetDuration("signalg.ping_timeout"),
		RedisDSN:                  v.GetString("redis.dsn"),
		RedisRouteTTL:             v.GetDuration("redis.route_ttl"),
		RedisPushChannel:          v.GetString("redis.push_channel"),
		RedisPushUsersChannel:     v.GetString("redis.push_users_channel"),
		RedisPushGroupChannel:     v.GetString("redis.push_group_channel"),
		RedisPushBroadcastChannel: v.GetString("redis.push_broadcast_channel"),
		JWTSecret:                 v.GetString("jwt.secret"),
		JWTExpiration:             v.GetDuration("jwt.expiration"),
	}

	cfg.normalize()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Defaults() Config {
	return Config{
		HTTPAddr:                  DefaultHTTPAddr,
		WebSocketPath:             DefaultWebSocketPath,
		GRPCSocket:                DefaultGRPCSocket,
		ShutdownTimeout:           DefaultShutdownTimeout,
		PingInterval:              DefaultPingInterval,
		PingTimeout:               DefaultPingTimeout,
		RedisDSN:                  DefaultRedisDSN,
		RedisRouteTTL:             DefaultRedisRouteTTL,
		RedisPushChannel:          DefaultRedisPushChannel,
		RedisPushUsersChannel:     DefaultRedisPushChannel + pushUsersSuffix,
		RedisPushGroupChannel:     DefaultRedisPushChannel + pushGroupSuffix,
		RedisPushBroadcastChannel: DefaultRedisPushChannel + pushBroadcastSuffix,
		JWTSecret:                 "",
		JWTExpiration:             time.Hour * 24 * 180,
	}
}

func setDefaults(v *viper.Viper) {
	defaults := Defaults()
	v.SetDefault("http.addr", defaults.HTTPAddr)
	v.SetDefault("websocket.path", defaults.WebSocketPath)
	v.SetDefault("grpc.socket", defaults.GRPCSocket)
	v.SetDefault("shutdown.timeout", defaults.ShutdownTimeout.String())
	v.SetDefault("signalg.ping_interval", defaults.PingInterval.String())
	v.SetDefault("signalg.ping_timeout", defaults.PingTimeout.String())
	v.SetDefault("redis.dsn", defaults.RedisDSN)
	v.SetDefault("redis.route_ttl", defaults.RedisRouteTTL.String())
	v.SetDefault("redis.push_channel", defaults.RedisPushChannel)
	v.SetDefault("redis.push_users_channel", defaults.RedisPushUsersChannel)
	v.SetDefault("redis.push_group_channel", defaults.RedisPushGroupChannel)
	v.SetDefault("redis.push_broadcast_channel", defaults.RedisPushBroadcastChannel)
	v.SetDefault("jwt.secret", defaults.JWTSecret)
	v.SetDefault("jwt.expiration", defaults.JWTExpiration.String())
}

func bindEnv(v *viper.Viper) {
	v.SetEnvPrefix("KIM_GATE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
	must(v.BindEnv("http.addr", "KIM_GATE_HTTP_ADDR"))
	must(v.BindEnv("websocket.path", "KIM_GATE_WS_PATH", "KIM_GATE_WEBSOCKET_PATH"))
	must(v.BindEnv("grpc.socket", "KIM_GATE_GRPC_SOCKET"))
	must(v.BindEnv("shutdown.timeout", "KIM_GATE_SHUTDOWN_TIMEOUT"))
	must(v.BindEnv("signalg.ping_interval", "KIM_GATE_PING_INTERVAL"))
	must(v.BindEnv("signalg.ping_timeout", "KIM_GATE_PING_TIMEOUT"))
	must(v.BindEnv("redis.dsn", "KIM_GATE_REDIS_DSN"))
	must(v.BindEnv("redis.route_ttl", "KIM_GATE_REDIS_ROUTE_TTL"))
	must(v.BindEnv("redis.push_channel", "KIM_GATE_REDIS_PUSH_CHANNEL"))
	must(v.BindEnv("redis.push_users_channel", "KIM_GATE_REDIS_PUSH_USERS_CHANNEL"))
	must(v.BindEnv("redis.push_group_channel", "KIM_GATE_REDIS_PUSH_GROUP_CHANNEL"))
	must(v.BindEnv("redis.push_broadcast_channel", "KIM_GATE_REDIS_PUSH_BROADCAST_CHANNEL"))
	must(v.BindEnv("jwt.secret", "KIM_GATE_JWT_SECRET"))
	must(v.BindEnv("jwt.expiration", "KIM_GATE_JWT_EXPIRATION"))
}

func bindFlags(v *viper.Viper, fs *pflag.FlagSet) error {
	bindings := map[string]string{
		"http.addr":                    "http-addr",
		"websocket.path":               "ws-path",
		"grpc.socket":                  "grpc-socket",
		"shutdown.timeout":             "shutdown-timeout",
		"signalg.ping_interval":        "ping-interval",
		"signalg.ping_timeout":         "ping-timeout",
		"redis.dsn":                    "redis-dsn",
		"redis.route_ttl":              "redis-route-ttl",
		"redis.push_channel":           "redis-push-channel",
		"redis.push_users_channel":     "redis-push-users-channel",
		"redis.push_group_channel":     "redis-push-group-channel",
		"redis.push_broadcast_channel": "redis-push-broadcast-channel",
		"jwt.secret":                   "jwt-secret",
		"jwt.expiration":               "jwt-expiration",
	}
	for key, name := range bindings {
		if err := v.BindPFlag(key, fs.Lookup(name)); err != nil {
			return fmt.Errorf("bind flag %s: %w", name, err)
		}
	}
	return nil
}

func readConfigFile(v *viper.Viper, configPath string) error {
	configPath = strings.TrimSpace(configPath)
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yml")
		v.AddConfigPath(".")
	}
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok && configPath == "" {
			return nil
		}
		return fmt.Errorf("read config file: %w", err)
	}
	return nil
}

func (c *Config) normalize() {
	c.JWTSecret = strings.TrimSpace(c.JWTSecret)
	c.HTTPAddr = strings.TrimSpace(c.HTTPAddr)
	c.WebSocketPath = normalizePath(c.WebSocketPath)
	c.GRPCSocket = strings.TrimSpace(c.GRPCSocket)
	c.RedisDSN = strings.TrimSpace(c.RedisDSN)
	c.RedisPushChannel = strings.TrimSpace(c.RedisPushChannel)
	c.RedisPushUsersChannel = strings.TrimSpace(c.RedisPushUsersChannel)
	c.RedisPushGroupChannel = strings.TrimSpace(c.RedisPushGroupChannel)
	c.RedisPushBroadcastChannel = strings.TrimSpace(c.RedisPushBroadcastChannel)
	c.normalizePushChannels()
}

func (c Config) Validate() error {
	if c.HTTPAddr == "" {
		return errors.New("http addr is required")
	}
	if c.WebSocketPath == "" {
		return errors.New("websocket path is required")
	}
	if c.GRPCSocket == "" {
		return errors.New("grpc socket path is required")
	}
	if c.ShutdownTimeout <= 0 {
		return errors.New("shutdown timeout must be positive")
	}
	if c.PingInterval < 0 {
		return errors.New("ping interval cannot be negative")
	}
	if c.PingTimeout < 0 {
		return errors.New("ping timeout cannot be negative")
	}
	if c.RedisDSN == "" {
		return errors.New("redis dsn is required")
	}
	if c.RedisRouteTTL <= 0 {
		return errors.New("redis route ttl must be positive")
	}
	if c.RedisPushChannel == "" {
		return errors.New("redis push channel is required")
	}
	if c.RedisPushUsersChannel == "" {
		return errors.New("redis users push channel is required")
	}
	if c.RedisPushGroupChannel == "" {
		return errors.New("redis group push channel is required")
	}
	if c.RedisPushBroadcastChannel == "" {
		return errors.New("redis broadcast push channel is required")
	}
	return nil
}

func (c *Config) normalizePushChannels() {
	if c == nil {
		return
	}
	base := strings.TrimSpace(c.RedisPushChannel)
	if base == "" {
		base = DefaultRedisPushChannel
		c.RedisPushChannel = base
	}
	if c.RedisPushUsersChannel == "" {
		c.RedisPushUsersChannel = base + pushUsersSuffix
	}
	if c.RedisPushGroupChannel == "" {
		c.RedisPushGroupChannel = base + pushGroupSuffix
	}
	if c.RedisPushBroadcastChannel == "" {
		c.RedisPushBroadcastChannel = base + pushBroadcastSuffix
	}
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return DefaultWebSocketPath
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func normalizeFlagArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	normalized := make([]string, len(args))
	for i, arg := range args {
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 2 {
			normalized[i] = "-" + arg
			continue
		}
		normalized[i] = arg
	}
	return normalized
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
