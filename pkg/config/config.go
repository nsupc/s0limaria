package config

import (
	"errors"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

type Telegram struct {
	Id  int    `yaml:"id"`
	Key string `yaml:"key"`
}

type Eurocore struct {
	Url      string `yaml:"url"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type Target struct {
	Region   string   `yaml:"region"`
	Telegram Telegram `yaml:"telegram"`
}

type Config struct {
	User            string   `yaml:"user"`
	Region          string   `yaml:"region"`
	Ratelimit       int      `yaml:"ratelimit"`
	Eurocore        Eurocore `yaml:"eurocore"`
	Targets         []Target `yaml:"targets"`
	DefaultTelegram Telegram `yaml:"default-telegram"`
}

func New(path string) (*Config, error) {
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
		if target.Telegram.Id == 0 || target.Telegram.Key == "" {
			config.Targets[idx].Telegram = config.DefaultTelegram
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

	if c.DefaultTelegram.Id == 0 || c.DefaultTelegram.Key == "" {
		return errors.New("all default-telegram attributes are required")
	}

	return nil
}

func (c *Config) Get(region string) (Target, bool) {
	for _, target := range c.Targets {
		if target.Region == region {
			return target, true
		}
	}

	return Target{}, false
}
