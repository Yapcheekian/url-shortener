package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
	migrate "github.com/rubenv/sql-migrate"
)

const (
	DB_USER     = "dcard_test"
	DB_PASSWORD = "mysecretpassword"
	DB_NAME     = "dcard_test"
)

func main() {
	dsn := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable", DB_USER, DB_PASSWORD, DB_NAME)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		fmt.Println("sql.Open failed", err.Error())
		os.Exit(1)
	}

	migrations := &migrate.FileMigrationSource{
		Dir: "./migrations",
	}

	_, err = migrate.Exec(db, "postgres", migrations, migrate.Up)
	if err != nil {
		fmt.Println("migrate failed: ", err.Error())
		os.Exit(1)
	}
}
