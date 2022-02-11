### Usage
```bash
./run.sh
```

### Data flow
![](./asset/shorten.png)


### Considerations
1. Redirection to original URL is made by responding back 301 permanent redirect. [301 status code](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/301) is chosed so that browser will cache the result and effectively lessen burden on server side. However, if business requirement changes, such that we would like to track click rate, then in such case [302 status code](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/302) will be a better choice.
2. As there are more read than write, result is stored in redis to improve performace.

### Limitation
1. For the time being, current implemention might cause id collision, even though it is unlikely. This is because the node bit is using random number from 0 to 10000000. To provide consistency, this is perhaps not accepted, thus, node ID should be computed using IP address to guarantee uniqueness. This will be put inside todo list.
```go
node, err := snowflake.NewNode(int64(rand.Intn(10000000)))
```
2. [Twitter snowflake](https://github.com/bwmarrin/snowflake/blob/master/snowflake.go#L17) use  Nov 04 2010 01:42:54 UTC as base time and [41 bits](https://github.com/bwmarrin/snowflake/blob/master/snowflake.go#L117) to store a timestamp. This mean the time bits will eventually running out of space (roughly on Year 2079) and there will be probability of collision occuring. When that time comes, a new epoch time is needed. We can mitigate this problem by using later base time at the beginning to prolong the expiry date, but this is not the ultimate solution.

### Dependency
- [postgres](https://www.postgresql.org/docs/) : advanced RDBMS
- [redis](https://redis.io/documentation) : fast key value store
- [gin](https://github.com/gin-gonic/gin) : fast http framework
- [sqlx](https://github.com/jmoiron/sqlx) : user-friendly version of `database/sql`
- [go-redis](https://github.com/go-redis/redis) : redis driver
- [snowflake](https://github.com/bwmarrin/snowflake) : generate unique ID
- [base62](https://github.com/mattheath/base62) : base62 conversion utility
