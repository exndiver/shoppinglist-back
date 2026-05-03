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
}

func Load() Config {
	_ = godotenv.Load()

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		log.Fatal(err)
	}

	return cfg
}
