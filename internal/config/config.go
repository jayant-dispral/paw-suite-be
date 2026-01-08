package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	ServerPort          string `mapstructure:"SERVER_PORT" env:"SERVER_PORT"`
	MongoDBDatabaseURI  string `mapstructure:"MONGODB_DATABASE_URI" env:"MONGODB_DATABASE_URI"`
	MongoDBDatabaseName string `mapstructure:"MONGODB_DATABASE_NAME" env:"MONGODB_DATABASE_NAME"`
	JWTSecret           string `mapstructure:"JWT_SECRET" env:"JWT_SECRET"`
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
	cfg := &Config{
		ServerPort: getEnv("SERVER_PORT", "8080"),
		//TODO:P1:fix the env fetching system NONE IS WORKING
		MongoDBDatabaseURI:  getEnv("MONGODB_DATABASE_URI", "mongodb://admin:password@localhost:27017/brand_threat?authSource=admin"),
		MongoDBDatabaseName: getEnv("MONGODB_DATABASE_NAME", "brand_threat"),
		JWTSecret:           getEnv("JWT_SECRET", "your-super-secret-jwt-key-change-this-in-production"),
	}
	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
