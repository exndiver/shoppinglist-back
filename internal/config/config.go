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

	RunMigrations  bool   `envconfig:"RUN_MIGRATIONS" default:"true"`
	MigrationsPath string `envconfig:"MIGRATIONS_PATH" default:"file://migrations"`

	ShutdownTimeout time.Duration `envconfig:"SHUTDOWN_TIMEOUT" default:"10s"`

	// Logging: JSON в файл по умолчанию logs/logs.log (от корня рабочей директории процесса).
	LogLevel   string `envconfig:"LOG_LEVEL" default:"info"`  // debug, info, warn, error
	LogFormat  string `envconfig:"LOG_FORMAT" default:"json"` // json | text
	LogService string `envconfig:"LOG_SERVICE" default:"shopping-backend"`
	LogFile    string `envconfig:"LOG_FILE" default:"logs/logs.log"`
	// LogSlowRequest: requests slower than this get level warn (0 disables).
	LogSlowRequest time.Duration `envconfig:"LOG_SLOW_REQUEST" default:"1s"`
}

func Load() Config {
	_ = godotenv.Load()

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		log.Fatal(err)
	}

	return cfg
}
