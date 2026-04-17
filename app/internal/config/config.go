package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type Config struct {
	App      App      `mapstructure:"app" validate:"required"`
	Database Database `mapstructure:"database" validate:"required"`
	Retry    Retry    `mapstructure:"retry" validate:"required"`
	Redis    Redis    `mapstructure:"redis" validate:"required"`
}

type App struct {
	Port            int           `mapstructure:"port" validate:"required,gte=1,lte=65535"`
	LogLevel        string        `mapstructure:"log_level" validate:"required,oneof=debug info warn error"`
	IsProd          bool          `mapstructure:"is_prod"`
	MigrationDir    string        `mapstructure:"migration_dir" validate:"required"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout" validate:"required,gt=0"`
	Services        Services      `mapstructure:"services" validate:"required"`
	Page            Page          `mapstructure:"page" validate:"required"`
}

type Redis struct {
	Host     string        `mapstructure:"host" validate:"required"`
	Port     int           `mapstructure:"port" validate:"required,gte=1,lte=65535"`
	Password string        `mapstructure:"password" validate:"required"`
	DB       int           `mapstructure:"db" validate:"gte=0"`
	Size     int           `mapstructure:"size" validate:"required,gte=1"`
	Duration time.Duration `mapstructure:"duration" validate:"required,gt=0"`
}

type Services struct {
	Auth          Auth          `mapstructure:"auth" validate:"required"`
	Conference    Conference    `mapstructure:"conference" validate:"required"`
	SlotGenerator SlotGenerator `mapstructure:"slot_generator" validate:"required"`
	SlotDuration  time.Duration `mapstructure:"slot_duration" validate:"required,gt=0"`
	PasswordCost  int           `mapstructure:"password_cost" validate:"required,gte=4,lte=31"`
}

type SlotGenerator struct {
	LookAhead int           `mapstructure:"lookahead" validate:"required,gt=0"`
	Interval  time.Duration `mapstructure:"interval" validate:"required,gt=0"`
}

type Auth struct {
	SecretKey     string        `mapstructure:"secret_key" validate:"required,min=32"`
	TokenDuration time.Duration `mapstructure:"token_duration" validate:"required,gt=0"`
}

type Conference struct {
	Port           int           `mapstructure:"port" validate:"required,gte=1,lte=65535"`
	RequestTimeout time.Duration `mapstructure:"request_timeout" validate:"required,gt=0"`
}

type Page struct {
	Default uint64 `mapstructure:"default" validate:"required,gt=0"`
	Min     uint64 `mapstructure:"min" validate:"required,gt=0"`
	Size    Size   `mapstructure:"size" validate:"required"`
}

type Size struct {
	Default uint64 `mapstructure:"default" validate:"required,gt=0"`
	Max     uint64 `mapstructure:"max" validate:"required,gt=0"`
	Min     uint64 `mapstructure:"min" validate:"required,gt=0"`
}

type Database struct {
	Dsn  string `mapstructure:"dsn" validate:"required"`
	Conn Conn   `mapstructure:"conn" validate:"required"`
}

type Conn struct {
	Max            int32         `mapstructure:"max" validate:"required,gt=0"`
	Min            int32         `mapstructure:"min" validate:"required,gte=0"`
	MaxLifeTime    time.Duration `mapstructure:"max_life_time" validate:"required,gt=0"`
	MaxIdleTime    time.Duration `mapstructure:"max_idle_time" validate:"required,gt=0"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout" validate:"required,gt=0"`
}

type Retry struct {
	Backoff     string        `mapstructure:"backoff" validate:"required,oneof=fixed linear exponential"`
	Base        time.Duration `mapstructure:"base" validate:"required,gt=0"`
	Factor      float64       `mapstructure:"factor" validate:"required,gt=1"`
	Max         time.Duration `mapstructure:"max" validate:"required,gt=0"`
	MaxAttempts int           `mapstructure:"max_attempts" validate:"required,gt=0,lte=10"`
	Jitter      float64       `mapstructure:"jitter" validate:"gte=0,lte=1"`
}

func Load(configFilePath string) (*Config, error) {
	v := viper.New()

	setDefaults(v)

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if configFilePath != "" {
		v.SetConfigFile(configFilePath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	return validator.New().Struct(c)
}
func setDefaults(v *viper.Viper) {
	v.SetDefault("app.port", 8080)
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.is_prod", false)
	v.SetDefault("app.shutdown_timeout", "10s")

	v.SetDefault("database.conn.max", 10)
	v.SetDefault("database.conn.connect_timeout", "5s")

	v.SetDefault("retry.max_attempts", 3)
	v.SetDefault("retry.factor", 2.0)

	v.SetDefault("redis.size", 1000)
	v.SetDefault("redis.duration", "30s")

	v.SetDefault("app.services.password_cost", 12)
	v.SetDefault("app.services.slot_duration", "30m")
	v.SetDefault("app.services.auth.token_duration", "24h")
	v.SetDefault("app.services.conference.port", 9090)

	v.SetDefault("app.services.slot_generator.lookahead", 30)
	v.SetDefault("app.services.slot_generator.interval", "1h")
}
