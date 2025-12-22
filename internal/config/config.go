package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	ServerPort         string `mapstructure:"SERVER_PORT" env:"SERVER_PORT"`
	MongoDBDatabaseURI string `mapstructure:"MONGODB_DATABASE_URI" env:"MONGODB_DATABASE_URI"`
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}

func Load() (*Config, error) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev" //default to dev
	}

	root, err := findProjectRoot()
	if err != nil {
		log.Printf("Warning: Could not find project root, using system env vars: %v", err)
		viper.AutomaticEnv()
		var cfg Config
		if err := viper.Unmarshal(&cfg); err != nil {
			return nil, err
		}
		return &cfg, nil
	}

	envFile := filepath.Join(root, ".env."+env)

	// Load .env file using viper
	viper.SetConfigFile(envFile)
	viper.SetConfigType("env")
	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: %s not found, using system env vars: %v", envFile, err)
	}
	viper.AutomaticEnv()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
