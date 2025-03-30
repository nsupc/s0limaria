package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Telegram struct {
	Id  int
	Key string
}

type Config struct {
	User             string
	Region           string
	Ratelimit        int
	EurocoreUrl      string `yaml:"eurocore_url"`
	EurocoreUser     string `yaml:"eurocore_user"`
	EurocorePassword string `yaml:"eurocore_password"`
	Targets          []string
	Telegram         Telegram
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

	return &config, nil
}
