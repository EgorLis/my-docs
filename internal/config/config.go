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
}

// String реализует интерфейс Stringer
func (c *Config) String() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  DBHost: %s\n", c.DBHost))
	sb.WriteString(fmt.Sprintf("  DBPort: %d\n", c.DBPort))
	sb.WriteString(fmt.Sprintf("  DBUser: %s\n", c.DBUser))
	sb.WriteString(fmt.Sprintf("  DBName: %s\n", c.DBName))
	sb.WriteString(fmt.Sprintf("  DBScheme : %s\n", c.DBScheme))
	sb.WriteString(fmt.Sprintf("  AppPort: %s\n", c.AppPort))

	// Пароль обычно маскируют в логах
	if c.DBPassword != "" {
		sb.WriteString("  DBPassword: ********\n")
	} else {
		sb.WriteString("  DBPassword: (empty)\n")
	}

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
	v.AutomaticEnv() // тянем переменные из окружения контейнера

	// регистрируем интересующие ключи окружения
	keys := []string{
		"APP_ENV", "APP_PORT",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SCHEME",
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
