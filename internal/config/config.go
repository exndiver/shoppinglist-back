package config

import (
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port string `envconfig:"PORT" default:"8080"`

	DBHost     string `envconfig:"DB_HOST" default:"localhost"`
	DBPort     string `envconfig:"DB_PORT" default:"5432"`
	DBUser     string `envconfig:"DB_USER" default:"shopping"`
	DBPassword string `envconfig:"DB_PASSWORD" default:"shopping"`
	DBName     string `envconfig:"DB_NAME" default:"shopping"`
	DBSSLMode  string `envconfig:"DB_SSLMODE" default:"disable"`

	DBURL string `envconfig:"DB_URL"`

	// Пул соединений (по умолчанию консервативные значения для старта).
	DBMaxConns        int32         `envconfig:"DB_MAX_CONNS" default:"10"`
	DBMinConns        int32         `envconfig:"DB_MIN_CONNS" default:"0"`
	DBMaxConnLifetime time.Duration `envconfig:"DB_MAX_CONN_LIFETIME" default:"30m"`
	DBMaxConnIdleTime time.Duration `envconfig:"DB_MAX_CONN_IDLE_TIME" default:"5m"`
	// Таймаут на каждый SQL-запрос (0 — не ограничивать, использовать ctx запроса).
	DBQueryTimeout time.Duration `envconfig:"DB_QUERY_TIMEOUT" default:"5s"`

	RunMigrations  bool   `envconfig:"RUN_MIGRATIONS" default:"true"`
	MigrationsPath string `envconfig:"MIGRATIONS_PATH" default:"file://migrations"`

	ShutdownTimeout time.Duration `envconfig:"SHUTDOWN_TIMEOUT" default:"10s"`

	// HTTP
	// Максимальный размер JSON-тела запроса в байтах (0 — без лимита).
	HTTPMaxBodyBytes int64 `envconfig:"HTTP_MAX_BODY_BYTES" default:"1048576"`
	// Таймаут на чтение всего тела запроса (0 — без лимита).
	HTTPReadTimeout  time.Duration `envconfig:"HTTP_READ_TIMEOUT" default:"0"`
	HTTPWriteTimeout time.Duration `envconfig:"HTTP_WRITE_TIMEOUT" default:"0"`

	// Logging
	LogLevel   string `envconfig:"LOG_LEVEL" default:"info"`  // debug, info, warn, error
	LogFormat  string `envconfig:"LOG_FORMAT" default:"json"` // json | text
	LogService string `envconfig:"LOG_SERVICE" default:"shopping-backend"`
	LogFile    string `envconfig:"LOG_FILE" default:"logs/logs.log"`
	// Куда писать логи: file | stdout | both. В Docker рекомендуется stdout/both.
	LogOutput string `envconfig:"LOG_OUTPUT" default:"file"`
	// Запросы медленнее этого порога пишутся уровнем warn (0 отключает).
	LogSlowRequest time.Duration `envconfig:"LOG_SLOW_REQUEST" default:"1s"`

	// Metrics: /metrics в формате Prometheus text exposition.
	MetricsEnabled bool   `envconfig:"METRICS_ENABLED" default:"true"`
	MetricsPath    string `envconfig:"METRICS_PATH" default:"/metrics"`

	// Rate limit: простое in-memory ограничение (token bucket) на весь API.
	// 0 отключает. RATE_LIMIT — среднее число запросов в секунду.
	RateLimit      float64 `envconfig:"RATE_LIMIT" default:"0"`
	RateLimitBurst int     `envconfig:"RATE_LIMIT_BURST" default:"0"`

	// Метаданные процесса.
	AppVersion string `envconfig:"APP_VERSION" default:"dev"`
}

func Load() Config {
	_ = godotenv.Load()

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		log.Fatal(err)
	}

	return cfg
}
