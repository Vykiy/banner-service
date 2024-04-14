package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

const (
	username = "banner_service"
	password = "bs1234"
	database = "banners"
)

func main() {
	m, err := migrate.New(
		"file://migrations",
		fmt.Sprintf("postgres://%s:%s@localhost:5432/%s?sslmode=disable", username, password, database))
	if err != nil {
		log.Fatal(err)
	}
	if err := m.Up(); err != nil {
		log.Fatal(err)
	}

	db, err := sqlx.Connect("postgres", fmt.Sprintf("user=%s dbname=%s password=%s sslmode=disable", username, database, password))
	if err != nil {
		log.Fatalln(err)
	}

	cache := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	s := NewServer(db, cache)

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Fatal error:", err)
	}

}
