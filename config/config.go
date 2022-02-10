package config

import (
	"log"
	"os"

	"github.com/spf13/viper"
)

func init() {
	viper.AddConfigPath("./config")
	viper.SetConfigType("yaml")
	switch env := os.Getenv("APP_ENV"); env {
	case "docker":
		viper.SetConfigName("docker")
	default:
		viper.SetConfigName("local")
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal(err)
	}
}
