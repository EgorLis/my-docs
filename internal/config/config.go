package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	DBHost     string `mapstructure:"DB_HOST"`
	DBPort     int    `mapstructure:"DB_PORT"`
	DBUser     string `mapstructure:"DB_USER"`
	DBPassword string `mapstructure:"DB_PASSWORD"`
	DBName     string `mapstructure:"DB_NAME"`
	DBScheme   string `mapstructure:"DB_SCHEME"`
	AppPort    string `mapstructure:"APP_PORT"`

	// --- S3 ---
	S3Endpoint  string `mapstructure:"S3_ENDPOINT"`
	S3Region    string `mapstructure:"S3_REGION"`
	S3Bucket    string `mapstructure:"S3_BUCKET"`
	S3AccessKey string `mapstructure:"S3_ACCESS_KEY"`
	S3SecretKey string `mapstructure:"S3_SECRET_KEY"`
	S3UseSSL    bool   `mapstructure:"S3_USE_SSL"`
	S3PathStyle bool   `mapstructure:"S3_PATH_STYLE"`
}

// String реализует интерфейс Stringer
func (c *Config) String() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  DBHost: %s\n", c.DBHost))
	sb.WriteString(fmt.Sprintf("  DBPort: %d\n", c.DBPort))
	sb.WriteString(fmt.Sprintf("  DBUser: %s\n", c.DBUser))
	sb.WriteString(fmt.Sprintf("  DBName: %s\n", c.DBName))
	sb.WriteString(fmt.Sprintf("  DBScheme: %s\n", c.DBScheme))
	sb.WriteString(fmt.Sprintf("  AppPort: %s\n", c.AppPort))

	// пароль маскируем
	if c.DBPassword != "" {
		sb.WriteString("  DBPassword: ********\n")
	} else {
		sb.WriteString("  DBPassword: (empty)\n")
	}

	// S3
	sb.WriteString(fmt.Sprintf("  S3Endpoint: %s\n", c.S3Endpoint))
	sb.WriteString(fmt.Sprintf("  S3Region: %s\n", c.S3Region))
	sb.WriteString(fmt.Sprintf("  S3Bucket: %s\n", c.S3Bucket))
	if c.S3AccessKey != "" {
		sb.WriteString("  S3AccessKey: ********\n")
	} else {
		sb.WriteString("  S3AccessKey: (empty)\n")
	}
	if c.S3SecretKey != "" {
		sb.WriteString("  S3SecretKey: ********\n")
	} else {
		sb.WriteString("  S3SecretKey: (empty)\n")
	}
	sb.WriteString(fmt.Sprintf("  S3UseSSL: %v\n", c.S3UseSSL))
	sb.WriteString(fmt.Sprintf("  S3PathStyle: %v\n", c.S3PathStyle))

	return sb.String()
}

// LoadFromEnv загружает конфигурацию из переменных окружения
func LoadFromEnv() (*Config, error) {
	// Загружаем .env только для локальной разработки
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(".env"); err != nil {
			return nil, errors.New("failed to load .env")
		}
	}

	v := viper.New()
	v.AutomaticEnv()

	// Регистрируем интересующие ключи окружения
	keys := []string{
		"APP_ENV", "APP_PORT",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SCHEME",
		"S3_ENDPOINT", "S3_REGION", "S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY",
		"S3_USE_SSL", "S3_PATH_STYLE",
	}
	for _, k := range keys {
		_ = v.BindEnv(k)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) GetDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.DBUser,
		c.DBPassword,
		c.DBHost,
		c.DBPort,
		c.DBName,
	)
}
