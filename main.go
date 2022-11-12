package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			MessageKey: "message",
			TimeKey:    "timestamp",
			EncodeTime: zapcore.RFC3339NanoTimeEncoder,
		}),
		zapcore.AddSync(os.Stdout),
		zapcore.InfoLevel,
	))
	defer logger.Sync()

	logger.Info("starting the server...")

	cfg, err := loadConfig()
	if err != nil {
		logger.Panic("failed to load config", zap.Error(err))
	}

	testRedis(logger, cfg)

	mux := http.NewServeMux()
	setupRoutes(mux, logger)

	srv := http.Server{
		Addr:              cfg.ServerAddress,
		Handler:           mux,
		ReadTimeout:       cfg.ServerReadTimeout,
		ReadHeaderTimeout: cfg.ServerReadTimeout,
		WriteTimeout:      cfg.ServerWriteTimeout,
	}

	go func() {
		logger.Info("server listening", zap.String("address", cfg.ServerAddress))

		if listenErr := srv.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			logger.Error("failed to listen and serve", zap.Error(listenErr))
		}
	}()

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
	<-shutdownCh

	logger.Info("shutting down the server...")

	const shutdownTimeout = 5 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if shutErr := srv.Shutdown(ctx); shutErr != nil {
		logger.Error("failed to shutting down the server", zap.Error(shutErr))
	}
}

func setupRoutes(mux *http.ServeMux, logger *zap.Logger) {
	// hello world
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello World!\n"))
		logger.Info("Hello World!")
	})

	// readiness probe
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// liveness probe
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func testRedis(logger *zap.Logger, cfg Config) {
	cl := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
	})

	if err := cl.Ping(context.Background()).Err(); err != nil {
		logger.Panic("failed to ping redis client", zap.Error(err))
	}

	const (
		key   = "demo"
		value = "test"
	)

	if err := cl.Set(context.Background(), key, value, 0).Err(); err != nil {
		logger.Panic("failed to set redis key", zap.Error(err))
	}

	val, err := cl.Get(context.Background(), key).Result()
	if err != nil {
		logger.Panic("failed to get redis key", zap.Error(err))
	}

	logger.Info("getting data from redis", zap.String(key, val))
}

type Config struct {
	ServerAddress      string        `envconfig:"SERVER_ADDRESS" default:"0.0.0.0:3000"`
	ServerReadTimeout  time.Duration `envconfig:"SERVER_READ_TIMEOUT" default:"5s"`
	ServerWriteTimeout time.Duration `envconfig:"SERVER_WRITE_TIMEOUT" default:"15s"`

	RedisAddress  string `envconfig:"REDIS_ADDRESS" default:"0.0.0.0:6379"`
	RedisPassword string `envconfig:"REDIS_PASSWORD" required:"true"`
}

func loadConfig() (Config, error) {
	var cfg Config

	err := envconfig.Process("", &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}
