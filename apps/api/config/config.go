package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// This function will Load the ENVIORNMENT VARIABLES from .env if GO_ENV variable is not set
func LoadENV() error {
	goEnv := os.Getenv("GO_ENV")

	if goEnv == "" || goEnv == "development" {
		err := godotenv.Load()
		if err != nil {
			return err
		}
	}

	return nil
}

type EnviornmentVariable struct {
	// All variables
	GO_ENV       string
	DB_USER_NAME string
	DB_PASSWORD  string
	DB_NAME      string
	DB_HOST      string
	DB_PORT      string
	DB_SSL_MODE  string
	PORT         int
	// JWT Configuration
	JWT_SECRET string
	JWT_ISSUER string
	// Redis Configuration
	REDIS_URL      string
	REDIS_PASSWORD string
	REDIS_DB       string
	// DigitalOcean Configuration
	DIGITALOCEAN_TOKEN string
	DO_SPACES_BUCKET   string
	DO_SPACES_REGION   string
	DO_SPACES_ENDPOINT string
	MODEL_ACCESS_KEY   string
}

func Get() (*EnviornmentVariable, error) {

	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		port = 8080
	}

	// Database defaults
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	envVariables := &EnviornmentVariable{
		GO_ENV:       os.Getenv("GO_ENV"),
		DB_USER_NAME: os.Getenv("DB_USER_NAME"),
		DB_PASSWORD:  os.Getenv("DB_PASSWORD"),
		DB_NAME:      os.Getenv("DB_NAME"),
		DB_HOST:      dbHost,
		DB_PORT:      dbPort,
		DB_SSL_MODE:  os.Getenv("DB_SSL_MODE"),
		PORT:         port,
		// JWT
		JWT_SECRET: os.Getenv("JWT_SECRET"),
		JWT_ISSUER: os.Getenv("JWT_ISSUER"),
		// Redis
		REDIS_URL:      os.Getenv("REDIS_URL"),
		REDIS_PASSWORD: os.Getenv("REDIS_PASSWORD"),
		REDIS_DB:       os.Getenv("REDIS_DB"),
		// DigitalOcean
		DIGITALOCEAN_TOKEN: os.Getenv("DIGITALOCEAN_TOKEN"),
		DO_SPACES_BUCKET:   os.Getenv("DO_SPACES_BUCKET"),
		DO_SPACES_REGION:   os.Getenv("DO_SPACES_REGION"),
		DO_SPACES_ENDPOINT: os.Getenv("DO_SPACES_ENDPOINT"),
		MODEL_ACCESS_KEY:   os.Getenv("MODEL_ACCESS_KEY"),
	}

	return envVariables, nil
}
