package models

type URL struct {
	ID        int64  `sql:"id"`
	ShortURL  string `sql:"short_url"`
	LongURL   string `sql:"long_url"`
	ExpireAt  string `sql:"expire_at"`
	CreatedAt string `sql:"created_at"`
}
