package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-redis/redis/v8"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Prometheus metrics.
var (
	counter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "hello_request_total",
		Help: "Number of requests",
	})

	histogram *prometheus.HistogramVec = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_duration",
			Help:    "HTTP server request duration histogram in milliseconds",
			Buckets: []float64{0.5, 1, 5, 10, 25, 50, 100, 300, 500, 1000, 5000},
			ConstLabels: map[string]string{
				"http_scheme": "http",
			},
		},
		[]string{"http_server_name", "http_route", "http_method", "http_status_code"},
	)
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

	client, err := connectRedis(cfg)
	if err != nil {
		logger.Panic("failed to connect with redis", zap.Error(err))
	}
	defer client.Close()

	r := chi.NewRouter()
	setupRoutes(r, logger, client)

	srv := http.Server{
		Addr:              cfg.ServerAddress,
		Handler:           r,
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

// Redis Request and Response objects.

type setKeyRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type getKeyResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func setupRoutes(r chi.Router, logger *zap.Logger, cl *redis.Client) {
	r.Group(func(r chi.Router) {
		r.Use(promMetricsMiddleware)

		// hello world
		r.Get("/hello", helloWorld(logger))

		// redis set key
		r.Post("/redis/set", redisSetKey(cl, logger))

		// redis get key
		r.Get("/redis/get/{key}", redisGetKey(cl, logger))
	})

	// readiness probe
	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// liveness probe
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// metrics
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
}

func helloWorld(logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		counter.Inc()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello World!\n"))
		logger.Info("Hello World!")
	}
}

func redisSetKey(cl *redis.Client, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req setKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Error("failed to decode request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := cl.Set(r.Context(), req.Key, req.Value, 0).Err(); err != nil {
			logger.Error("failed to set key", zap.Error(err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)

		logger.Info("key added successfully!")
	}
}

func redisGetKey(cl *redis.Client, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := chi.URLParam(r, "key")

		val, err := cl.Get(r.Context(), key).Result()
		if err != nil {
			logger.Error("failed to get key", zap.Error(err))

			if errors.Is(err, redis.Nil) {
				http.Error(w, "key not found", http.StatusNotFound)
				return
			}

			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resp := getKeyResponse{
			Key:   key,
			Value: val,
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)

		logger.Info("key retrieved successfully!")
	}
}

func promMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		rec := &recorder{
			ResponseWriter: w,
		}

		next.ServeHTTP(rec, r)

		histogram.
			WithLabelValues("http-server-demo", r.URL.Path, r.Method, strconv.Itoa(rec.status)).
			Observe(float64(time.Since(startedAt)) / float64(time.Millisecond))
	})
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
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

func connectRedis(cfg Config) (*redis.Client, error) {
	cl := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
	})

	if err := cl.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("pinging redis client: %w", err)
	}

	return cl, nil
}
