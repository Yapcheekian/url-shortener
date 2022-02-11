### Usage
```bash
./run.sh
```

### Data flow
![](./asset/shorten.png)

### Dependency
- [postgres](https://www.postgresql.org/docs/) : advanced RDBMS
- [redis](https://redis.io/documentation) : fast key value store
- [gin](https://github.com/gin-gonic/gin) : fast http framework
- [pgx](https://github.com/jackc/pgx) : user-friendly version of `database/sql`
- [go-redis](https://github.com/go-redis/redis) : redis driver
- [snowflake](https://github.com/bwmarrin/snowflake) : generate unique ID
- [base62](https://github.com/mattheath/base62) : base62 conversion utility
