package middlewares

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

func RateLimit(client *redis.Client, expiration time.Duration, maxVisitCount int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		newVisitCount, ttl, err := limit(ip, client, expiration)
		if err != nil {
			c.Abort()
			return
		}

		remaining := maxVisitCount - newVisitCount
		if remaining < 0 {
			remaining = 0
		}
		c.Writer.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		c.Writer.Header().Set("X-RateLimit-Reset", strconv.Itoa(int(ttl.Seconds())))
		if newVisitCount > maxVisitCount {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		c.Next()
	}
}

func limit(ipAddr string, client *redis.Client, expiration time.Duration) (int64, time.Duration, error) {
	pipe := client.Pipeline()
	pipedCmds := []interface{}{
		pipe.SetNX(context.TODO(), ipAddr, 0, expiration),
		pipe.Incr(context.TODO(), ipAddr),
		pipe.TTL(context.TODO(), ipAddr),
	}
	_, err := pipe.Exec(context.TODO())
	if err != nil {
		return 0, 0, err
	}

	executedSetVisitCountNX := pipedCmds[0].(*redis.BoolCmd)
	executedIncrVisitCountByIP := pipedCmds[1].(*redis.IntCmd)
	executedGetTTL := pipedCmds[2].(*redis.DurationCmd)

	var newCount int64
	var ttl time.Duration

	if err = executedSetVisitCountNX.Err(); err != nil {
		return 0, 0, err
	}
	if newCount, err = executedIncrVisitCountByIP.Result(); err != nil {
		return 0, 0, err
	}
	if ttl, err = executedGetTTL.Result(); err != nil {
		return 0, 0, err
	}
	return newCount, ttl, nil
}
