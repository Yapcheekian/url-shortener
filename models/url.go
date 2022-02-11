package models

type URL struct {
	ID        int64  `db:"id"`
	ShortURL  string `db:"short_url"`
	LongURL   string `db:"long_url"`
	ExpireAt  string `db:"expire_at"`
	CreatedAt string `db:"created_at"`
}
