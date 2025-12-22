package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	ServerPort         string `mapstructure:"SERVER_PORT" env:"SERVER_PORT"`
	MongoDBDatabaseURI string `mapstructure:"MONGODB_DATABASE_URI" env:"MONGODB_DATABASE_URI"`
}

func Load() (*Config, error) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev" //default to dev
	}
	envFile := ".env" + env
	if err := godotenv.Load(envFile); err != nil {
		log.Printf("Warning: %s not found, using system env vars", envFile)
	}
	viper.AutomaticEnv()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
