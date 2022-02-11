package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type URLSuite struct {
	suite.Suite
	router *gin.Engine

	mockSql sqlmock.Sqlmock
}

func (s *URLSuite) SetupTest() {
	gin.SetMode(gin.TestMode)
	s.router = gin.New()

	r := s.router.Group("/")
	mock, sqlMock, err := sqlmock.New()
	s.NoError(err)
	s.mockSql = sqlMock
	mockDB := sqlx.NewDb(mock, "postgres")
	NewShortenerHandler(r, mockDB, nil)
}

func TestURLSuite(t *testing.T) {
	suite.Run(t, new(URLSuite))
}

func (s *URLSuite) TestShortenURL() {
	tt := []struct {
		desc          string
		path          string
		method        string
		expStatusCode int
		mockFunc      func()
		reqBody       string
	}{
		{
			desc:          "longURL already exist in db",
			path:          "/api/v1/urls",
			method:        http.MethodPost,
			expStatusCode: http.StatusOK,
			mockFunc: func() {
				mockRows := sqlmock.NewRows([]string{"id", "short_url", "expire_at"}).AddRow(12345, "qwerty", "2030-02-20T09:20:41+08:00")
				s.mockSql.ExpectQuery(regexp.QuoteMeta("SELECT id, short_url, expire_at FROM urls WHERE long_url = $1")).WithArgs("https://amazon.com").WillReturnRows(mockRows)

			},
			reqBody: `{"url": "https://amazon.com", "expireAt": "2025-02-20T09:20:41+08:00"}`,
		},
		{
			desc:          "longURL already exist in db but input date is newer than the old one",
			path:          "/api/v1/urls",
			method:        http.MethodPost,
			expStatusCode: http.StatusOK,
			mockFunc: func() {
				mockRows := sqlmock.NewRows([]string{"id", "short_url", "expire_at"}).AddRow(12345, "qwerty", "2025-02-20T09:20:41+08:00")
				s.mockSql.ExpectQuery(regexp.QuoteMeta("SELECT id, short_url, expire_at FROM urls WHERE long_url = $1")).WithArgs("https://amazon.com").WillReturnRows(mockRows)
				s.mockSql.ExpectExec(regexp.QuoteMeta("UPDATE urls SET expire_at = $1 WHERE id = $2")).WithArgs("2030-02-20T09:20:41+08:00", 12345).WillReturnResult(sqlmock.NewResult(1, 1))
			},
			reqBody: `{"url": "https://amazon.com", "expireAt": "2030-02-20T09:20:41+08:00"}`,
		},
		{
			desc:          "input date is behind current timestamp",
			path:          "/api/v1/urls",
			method:        http.MethodPost,
			expStatusCode: http.StatusInternalServerError,
			mockFunc:      func() {},
			reqBody:       `{"url": "https://amazon.com", "expireAt": "2010-02-20T09:20:41+08:00"}`,
		},
		{
			desc:          "input URL is malformed",
			path:          "/api/v1/urls",
			method:        http.MethodPost,
			expStatusCode: http.StatusInternalServerError,
			mockFunc:      func() {},
			reqBody:       `{"url": "https://amazon||||com", "expireAt": "2030-02-20T09:20:41+08:00"}`,
		},
		{
			desc:          "longURL does not exist in db",
			path:          "/api/v1/urls",
			method:        http.MethodPost,
			expStatusCode: http.StatusOK,
			mockFunc: func() {
				mockRows := sqlmock.NewRows([]string{"id", "short_url", "expire_at"}).RowError(0, sql.ErrNoRows)
				s.mockSql.ExpectQuery(regexp.QuoteMeta("SELECT id, short_url, expire_at FROM urls WHERE long_url = $1")).WithArgs("https://amazon.com").WillReturnRows(mockRows)

				generateID = func() (int64, error) {
					return 66666, nil
				}

				base62EncodeID = func(n int64) string {
					return "ASDFG"
				}
				s.mockSql.ExpectExec(regexp.QuoteMeta("INSERT INTO urls(id, short_url, long_url, expire_at) VALUES ($1, $2, $3, $4)")).WithArgs(66666, "ASDFG", "https://amazon.com", "2030-02-20T09:20:41+08:00").WillReturnResult(sqlmock.NewResult(1, 1))
			},
			reqBody: `{"url": "https://amazon.com", "expireAt": "2030-02-20T09:20:41+08:00"}`,
		},
	}

	for _, tc := range tt {
		s.Run(tc.desc, func() {
			tc.mockFunc()
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.reqBody))
			respRecorder := httptest.NewRecorder()
			s.router.ServeHTTP(respRecorder, req)
			result := respRecorder.Result()
			s.Require().Equal(tc.expStatusCode, result.StatusCode)
		})
	}
}

func TestParseServerHost(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/urls", strings.NewReader(`{"url": "https://amazon.com", "expireAt": "2030-02-20T09:20:41+08:00"}`))
	req.Host = "test.com"
	result := parseServerHost(req)
	assert.Equal(t, "http://test.com", result)
}

func (s *URLSuite) TestRedirectURL() {
	tt := []struct {
		desc          string
		path          string
		method        string
		expStatusCode int
		mockFunc      func()
	}{
		{
			desc:          "url exist in db",
			path:          "/QAZWSX",
			method:        http.MethodGet,
			expStatusCode: http.StatusMovedPermanently,
			mockFunc: func() {
				decodeBase62ToID = func(s string) int64 {
					return 12345
				}
				mockRows := sqlmock.NewRows([]string{"long_url", "expire_at"}).AddRow("https://amazon.com", "2030-02-20T09:20:41+08:00")
				s.mockSql.ExpectQuery(regexp.QuoteMeta("SELECT long_url, expire_at FROM urls WHERE id = $1")).WithArgs(12345).WillReturnRows(mockRows)
			},
		},
		{
			desc:          "url does not exist in db",
			path:          "/QAZWSX",
			method:        http.MethodGet,
			expStatusCode: http.StatusNotFound,
			mockFunc: func() {
				decodeBase62ToID = func(s string) int64 {
					return 12345
				}
				mockRows := sqlmock.NewRows([]string{"long_url", "expire_at"}).RowError(0, sql.ErrNoRows)
				s.mockSql.ExpectQuery(regexp.QuoteMeta("SELECT long_url, expire_at FROM urls WHERE id = $1")).WithArgs(12345).WillReturnRows(mockRows)
			},
		},
		{
			desc:          "url exist in db but is expired",
			path:          "/QAZWSX",
			method:        http.MethodGet,
			expStatusCode: http.StatusNotFound,
			mockFunc: func() {
				decodeBase62ToID = func(s string) int64 {
					return 12345
				}
				mockRows := sqlmock.NewRows([]string{"long_url", "expire_at"}).AddRow("https://amazon.com", "2010-02-20T09:20:41+08:00")
				s.mockSql.ExpectQuery(regexp.QuoteMeta("SELECT long_url, expire_at FROM urls WHERE id = $1")).WithArgs(12345).WillReturnRows(mockRows)
			},
		},
	}

	for _, tc := range tt {
		s.Run(tc.desc, func() {
			tc.mockFunc()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			respRecorder := httptest.NewRecorder()
			s.router.ServeHTTP(respRecorder, req)
			result := respRecorder.Result()
			s.Require().Equal(tc.expStatusCode, result.StatusCode)
		})
	}
}
