package config

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	slogbetterstack "github.com/samber/slog-betterstack"
)

type Eurocore struct {
	Url      string `yaml:"url"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type Target struct {
	Region   string `yaml:"region"`
	Template string `yaml:"template"`
}

type Log struct {
	Level    string `yaml:"level"`
	Token    string `yaml:"token"`
	Endpoint string `yaml:"endpoint"`
}

type Config struct {
	User            string   `yaml:"user"`
	Region          string   `yaml:"region"`
	Ratelimit       int      `yaml:"ratelimit"`
	Eurocore        Eurocore `yaml:"eurocore"`
	Targets         []Target `yaml:"targets"`
	DefaultTemplate string   `yaml:"default-template"`
	Log             Log      `yaml:"log"`
	Heartbeat       string   `yaml:"heartbeat-url"`
}

func New() (*Config, error) {
	var path string

	if len(os.Args) > 1 {
		path = os.Args[1]
	} else {
		path = "config.yml"
	}

	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := Config{}

	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return nil, err
	}

	err = config.validate()
	if err != nil {
		return nil, err
	}

	for idx, target := range config.Targets {
		if target.Template == "" {
			config.Targets[idx].Template = config.DefaultTemplate
		}
	}

	return &config, nil
}

func (c *Config) validate() error {
	if c.User == "" {
		return errors.New("user is a required field")
	}

	if c.Region == "" {
		return errors.New("region is a required field")
	}

	if c.Ratelimit < 1 || c.Ratelimit > 50 {
		slog.Warn("invalid value for ratelimit; setting to 30", slog.Int("ratelimit", c.Ratelimit))
		c.Ratelimit = 30
	}

	if c.Eurocore.Url == "" || c.Eurocore.User == "" || c.Eurocore.Password == "" {
		return errors.New("all eurocore attributes are required")
	}

	if len(c.Targets) < 1 {
		return errors.New("at least one target region must be set")
	}

	for _, target := range c.Targets {
		if target.Region == "" {
			return errors.New("region is a required attribute for all targets")
		}
	}

	if c.DefaultTemplate == "" {
		return errors.New("default-template is required")
	}

	c.Log.Level = strings.ToLower(c.Log.Level)

	c.initLogger()
	c.startHeartbeat()

	return nil
}

func (c *Config) initLogger() {
	var logger *slog.Logger
	var logLevel slog.Level

	switch c.Log.Level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	if c.Log.Token != "" && c.Log.Endpoint != "" {
		logger = slog.New(slogbetterstack.Option{
			Token:    c.Log.Token,
			Endpoint: c.Log.Endpoint,
			Level:    logLevel,
		}.NewBetterstackHandler())
	} else {
		logger = slog.Default()
	}

	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(logLevel)
}

func (c *Config) Get(region string) (Target, bool) {
	for _, target := range c.Targets {
		if target.Region == region {
			return target, true
		}
	}

	return Target{}, false
}

func (c *Config) startHeartbeat() {
	if c.Heartbeat == "" {
		return
	}

	ticker := time.NewTicker(30 * time.Minute)

	go func() {
		for range ticker.C {
			_, err := http.Get(c.Heartbeat)
			if err != nil {
				slog.Error("heartbeat failed", slog.Any("error", err))
			}
		}
	}()
}
