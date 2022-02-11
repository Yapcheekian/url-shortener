package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/Yapcheekian/url-shortener/config"
	"github.com/Yapcheekian/url-shortener/handlers"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

var (
	shortenerHandler *handlers.ShortenerHandler
)

func init() {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		viper.GetString("DB_HOST"),
		viper.GetString("DB_USER"),
		viper.GetString("DB_PASS"),
		viper.GetString("DB_NAME"),
		viper.GetString("DB_PORT"),
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		panic(err)
	}

	if err := db.Ping(); err != nil {
		panic(err)
	}

	rClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", viper.GetString("REDIS_HOST"), viper.GetString("REDIS_PORT")),
	})

	if cmd := rClient.Ping(context.Background()); cmd.Err() != nil {
		panic(cmd.Err())
	}

	shortenerHandler = handlers.NewShortenerHandler(db, rClient)
}

func main() {
	r := gin.Default()

	v1 := r.Group("/api/v1")
	v1.POST("/urls", shortenerHandler.ShortenURL)

	r.GET("/:urlID", shortenerHandler.RedirectURL)

	svr := http.Server{
		Addr:    viper.GetString("APP_PORT"),
		Handler: r,
	}

	go func() {
		if err := svr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Fail to start server: ", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	// Relay incoming SIGTERM, SIGINT to quit
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	<-quit
	log.Println("Shutting down server...")

	// The context is used to inform the application it has 30 seconds to finish
	// cleaning up remaining resources
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := svr.Shutdown(ctx); err != nil {
		log.Println(fmt.Sprintf("Server forced to shutdown: %s", err.Error()))
	}
}
