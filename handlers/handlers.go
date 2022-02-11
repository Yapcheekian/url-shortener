package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/Yapcheekian/url-shortener/models"
	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	"github.com/mattheath/base62"
)

const (
	ttl = 10 * time.Minute
)

var (
	timeFormat = time.RFC3339 // ISO 8601
	regex      = regexp.MustCompile("^(http|https)://")
)

// generateID will output unique snowflake ID
// It will be monkey patch during
// testing to produce predictable result
var generateID = func() (int64, error) {
	node, err := snowflake.NewNode(1)
	if err != nil {
		return -1, err
	}

	return int64(node.Generate()), nil
}

var base62EncodeID = base62.EncodeInt64

type ShortenerHandler struct {
	db    *sqlx.DB
	redis *redis.Client
}

// NewShortenerHandler is the factory function of ShortenerHandler
func NewShortenerHandler(router *gin.RouterGroup, db *sqlx.DB, rClient *redis.Client) {
	handler := &ShortenerHandler{
		db:    db,
		redis: rClient,
	}

	v1 := router.Group("/api/v1")
	v1.POST("/urls", handler.ShortenURL)
	router.GET("/:urlID", handler.RedirectURL)
}

type urlRequest struct {
	URL      string `json:"url"`
	ExpireAt string `json:"expireAt"`
}

type urlResponse struct {
	ID       string `json:"id"`
	ShortURL string `json:"shortUrl"`
}

// ShortenURL first check whether uploaded URL exists, if not, shorten the uploaded URL
func (h *ShortenerHandler) ShortenURL(c *gin.Context) {
	var urlRequest urlRequest

	if err := c.ShouldBindJSON(&urlRequest); err != nil {
		log.Println("ShouldBindJSON failed: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	if err := validateInput(urlRequest); err != nil {
		log.Println("validateInput failed: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	url, err := h.findByLongURL(urlRequest.URL)
	if err != nil && err != sql.ErrNoRows {
		log.Println("findByLongURL failed: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	// uploaded URL exists
	if url.ShortURL != "" {
		// check whether existing url is expired
		// if not expired, check whether user would like to
		// update to newer time
		if !checkValidURL(url.ExpireAt, urlRequest.ExpireAt) {
			if err := h.updateURLExpireTime(url.ID, urlRequest.ExpireAt); err != nil {
				log.Println("updateURLExpireTime failed: ", err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
				return
			}
		}

		if err := h.redis.Set(context.TODO(), url.ShortURL, urlRequest.URL, ttl).Err(); err != nil {
			log.Println("redis.Set failed: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		host := parseServerHost(c.Request)
		c.JSON(http.StatusOK, urlResponse{
			ID:       url.ShortURL,
			ShortURL: fmt.Sprintf("%s/%s", host, url.ShortURL),
		})
		return
	}

	id, err := generateID()
	if err != nil {
		log.Println("generateID failed: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	shortURL := base62EncodeID(id)

	if err := h.insertNewURL(id, shortURL, urlRequest.URL, urlRequest.ExpireAt); err != nil {
		log.Println("insertNewURL failed: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	if err := h.redis.Set(context.TODO(), shortURL, urlRequest.URL, ttl).Err(); err != nil {
		log.Println("redis.Set failed: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	host := parseServerHost(c.Request)
	c.JSON(http.StatusOK, urlResponse{
		ID:       shortURL,
		ShortURL: fmt.Sprintf("%s/%s", host, shortURL),
	})
}

func (h *ShortenerHandler) RedirectURL(c *gin.Context) {
	shortURL := c.Param("urlID")

	val, err := h.redis.Get(context.TODO(), shortURL).Result()
	if err != nil && err != redis.Nil {
		log.Println("redis.Get failed: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	// cache hit
	if err != redis.Nil {
		c.Redirect(http.StatusMovedPermanently, val)
		return
	}

	url, err := h.findByShortURL(shortURL)

	if err != nil && err != sql.ErrNoRows {
		log.Println("findByShortURL failed: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	if url.LongURL == "" {
		c.JSON(http.StatusNotFound, gin.H{"message": "url not found"})
		return
	}

	// check expire time
	if isExpired(url.ExpireAt) {
		c.JSON(http.StatusNotFound, gin.H{"message": "url is expired"})
		return
	}

	c.Redirect(http.StatusMovedPermanently, url.LongURL)
}

func isExpired(expireTime string) bool {
	expire, _ := time.Parse(timeFormat, expireTime)

	return expire.Before(time.Now())
}

func (h *ShortenerHandler) updateURLExpireTime(id int64, expireTime string) error {
	if _, err := h.db.Exec("UPDATE urls SET expire_at = $1 WHERE id = $2", expireTime, id); err != nil {
		return err
	}

	return nil
}

func checkValidURL(expireTime string, inputTime string) bool {
	expire, _ := time.Parse(timeFormat, expireTime)

	input, _ := time.Parse(timeFormat, inputTime)

	return expire.After(time.Now()) && expire.After(input)
}

func parseServerHost(req *http.Request) string {
	var scheme string
	if req.TLS == nil {
		scheme = "http"
	} else {
		scheme = "https"
	}

	host := req.Host

	return scheme + "://" + host
}

func validateInput(urlReq urlRequest) error {
	expire, err := time.Parse(timeFormat, urlReq.ExpireAt)

	if err != nil {
		return err
	}

	if expire.Before(time.Now()) {
		return errors.New("expire time must be in the future")
	}

	//Trying to parse a hostname and path
	// without a scheme is invalid but may not necessarily return an
	// error, due to parsing ambiguities.
	if _, err := url.Parse(urlReq.URL); err != nil {
		return err
	}

	// net/url.Parse does not throw error even if
	// input url does not contains scheme
	// so we manually check it
	if !regex.MatchString(urlReq.URL) {
		return errors.New("scheme is required in URL format")
	}

	return nil
}

func (h *ShortenerHandler) findByShortURL(shortURL string) (models.URL, error) {
	var url models.URL

	if err := h.db.Get(&url, "SELECT long_url, expire_at FROM urls WHERE short_url = $1", shortURL); err != nil {
		return url, err
	}

	return url, nil
}

func (h *ShortenerHandler) findByLongURL(longURL string) (models.URL, error) {
	var url models.URL

	if err := h.db.Get(&url, "SELECT id, short_url, expire_at FROM urls WHERE long_url = $1", longURL); err != nil {
		return url, err
	}

	return url, nil
}

func (h *ShortenerHandler) insertNewURL(id int64, shortURL string, longURL string, expireAt string) error {
	_, err := h.db.Exec("INSERT INTO urls(id, short_url, long_url, expire_at) VALUES ($1, $2, $3, $4)", id, shortURL, longURL, expireAt)

	return err
}
