package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	"github.com/mattheath/base62"
)

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
func NewShortenerHandler(db *sqlx.DB, rClient *redis.Client) *ShortenerHandler {
	return &ShortenerHandler{
		db:    db,
		redis: rClient,
	}
}

type urlRequest struct {
	URL      string    `json:"url"`
	ExpireAt time.Time `json:"expireAt"`
}

type urlResponse struct {
	ID       int64  `json:"id"`
	ShortURL string `json:"shortUrl"`
}

func (h *ShortenerHandler) ShortenURL(c *gin.Context) {
	var urlRequest urlRequest

	if err := c.ShouldBindJSON(&urlRequest); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
	}

	found, err := h.checkURLExist(urlRequest.URL, urlRequest.ExpireAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
	}

	if !found {
		id, err := generateID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		}

		shortURL := base62.EncodeInt64(id)

		if err := h.insertNewURL(id, shortURL, urlRequest.URL, urlRequest.ExpireAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		}

		c.JSON(http.StatusOK, urlResponse{ID: id, ShortURL: shortURL})
	} else {
		response, err := h.getURLResponse(urlRequest.URL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		}

		c.JSON(http.StatusOK, response)
	}
}

func (h *ShortenerHandler) RedirectURL(c *gin.Context) {

}

func (h *ShortenerHandler) getURLResponse(url string) (urlResponse, error) {
	var urlResponse urlResponse

	if err := h.db.Get(&urlResponse, "SELECT short_url, expire_at FROM urls WHERE long_url = ?", url); err != nil {
		return urlResponse, err
	}

	return urlResponse, nil
}

func (h *ShortenerHandler) checkURLExist(url string, expireAt time.Time) (bool, error) {
	var exists bool

	query := fmt.Sprintf("SELECT exists (%s)", "SELECT * FROM urls WHERE long_url = ? AND expired_at < ?")

	if err := h.db.QueryRow(query, url, expireAt).Scan(&exists); err != nil && err != sql.ErrNoRows {
		return false, err
	}

	return exists, nil
}

func (h *ShortenerHandler) insertNewURL(id int64, shortURL string, longURL string, expireAt time.Time) error {
	_, err := h.db.Exec("INSERT INTO urls(id, short_url, long_url, expire_at) VALUES (?, ?, ?, ?)", id, shortURL, longURL, expireAt)

	return err
}
