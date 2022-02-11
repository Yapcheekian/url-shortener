package handlers

import (
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
	ID       string `json:"id" db:"id"`
	ShortURL string `json:"shortUrl" db:"short_url"`
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
		valid, err := checkValidURL(url.ExpireAt, urlRequest.ExpireAt)

		if err != nil {
			log.Println("checkValidURL failed: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		if !valid {
			if err := h.updateURLExpireTime(url.ID, urlRequest.ExpireAt); err != nil {
				log.Println("updateURLExpireTime failed: ", err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
				return
			}
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

	shortURL := base62.EncodeInt64(id)

	if err := h.insertNewURL(id, shortURL, urlRequest.URL, urlRequest.ExpireAt); err != nil {
		log.Println("insertNewURL failed: ", err)
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

}

func (h *ShortenerHandler) updateURLExpireTime(id int64, expireTime string) error {
	if _, err := h.db.Exec("UPDATE urls SET expire_at = $1 WHERE id = $2", expireTime, id); err != nil {
		return err
	}

	return nil
}

func checkValidURL(expireTime string, inputTime string) (bool, error) {
	expire, err := time.Parse(timeFormat, expireTime)
	if err != nil {
		return false, err
	}

	input, _ := time.Parse(timeFormat, inputTime)

	return expire.After(time.Now()) && expire.After(input), nil
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
