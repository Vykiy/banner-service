package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

var jwtKey = []byte("super_secret")

type claims struct {
	UserID  int  `json:"user_id"`
	IsAdmin bool `json:"is_admin"`
	jwt.Claims
}

type user struct {
	ID      int  `db:"id"`
	IsAdmin bool `db:"is_admin"`
}

type banner struct {
	ID         int    `db:"id"`
	Data       string `db:"data"`
	IsDisabled bool   `db:"is_disabled"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type server struct {
	db    *sqlx.DB
	cache *redis.Client
}

func NewServer(db *sqlx.DB, cache *redis.Client) *server {
	return &server{
		db:    db,
		cache: cache,
	}
}

func (s *server) getUserBannerHandler(w http.ResponseWriter, r *http.Request) {
	u, err := getUserFromRequest(r)
	if err != nil {
		http.Error(w, jsonError("Пользователь не авторизован"), http.StatusUnauthorized)
		return
	}

	tagID, err := strconv.Atoi(r.URL.Query().Get("tag_id"))
	if err != nil {
		http.Error(w, jsonError("tag_id должен быть числом"), http.StatusBadRequest)
		return
	}

	featureID, err := strconv.Atoi(r.URL.Query().Get("feature_id"))
	if err != nil {
		http.Error(w, jsonError("feature_id должен быть числом"), http.StatusBadRequest)
		return
	}

	var useLastRevision bool
	if lr := r.URL.Query().Get("use_last_revision"); lr != "" {
		useLastRevision, err = strconv.ParseBool(lr)
		if err != nil {
			http.Error(w, jsonError("use_last_revision должен быть булевским значением"), http.StatusBadRequest)
			return
		}
	}

	if !useLastRevision {
		val, err := s.cache.Get(context.Background(), fmt.Sprintf("%d-%d", tagID, featureID)).Result()
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(val); err != nil {
				http.Error(w, jsonError("Внутренняя ошибка сервера"), http.StatusInternalServerError)
			}
			return
		}
	}

	var b banner

	query := `SELECT id, data, is_disabled FROM public.banners 
	WHERE id = (SELECT banner_id FROM banner_feature_tags WHERE feature_id = $1 AND tag_id = $2)`

	if !u.IsAdmin {
		query += " AND is_disabled = FALSE"
	}

	if err := s.db.Get(&b,
		query, featureID, tagID); err != nil {

		log.Println("error while getting a banner from database: ", err)
		http.Error(w, jsonError("Внутренняя ошибка сервера"), http.StatusInternalServerError)
		return
	}

	if !b.IsDisabled {
		if err := s.cache.Set(context.Background(), fmt.Sprintf("%d-%d", tagID, featureID), b.Data, 5*time.Minute); err != nil {
			log.Println("error while writing banner data to cache: ", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(b.Data); err != nil {
		http.Error(w, jsonError("Внутренняя ошибка сервера"), http.StatusInternalServerError)
	}
}

func (s *server) getBannderHandler(w http.ResponseWriter, r *http.Request) {
	u, err := getUserFromRequest(r)
	if err != nil {
		http.Error(w, jsonError("Пользователь не авторизован"), http.StatusUnauthorized)
		return
	}

	if !u.IsAdmin {
		http.Error(w, jsonError("Пользователь не имеет доступа"), http.StatusForbidden)
		return
	}

	tagID, err := strconv.Atoi(r.URL.Query().Get("tag_id"))
	if err != nil {
		http.Error(w, jsonError("tag_id должен быть числом"), http.StatusBadRequest)
		return
	}

	featureID, err := strconv.Atoi(r.URL.Query().Get("feature_id"))
	if err != nil {
		http.Error(w, jsonError("feature_id должен быть числом"), http.StatusBadRequest)
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		http.Error(w, jsonError("limit должен быть числом"), http.StatusBadRequest)
		return
	}

	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil {
		http.Error(w, jsonError("offset должен быть числом"), http.StatusBadRequest)
		return
	}
}

func (s *server) generateTokenHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.Atoi(r.URL.Query().Get("user_id"))
	if err != nil {
		http.Error(w, jsonError("user_id должен быть числом"), http.StatusBadRequest)
		return
	}

	var u user

	if err := s.db.Get(&u, `SELECT id, is_admin FROM public.users WHERE id = $1`, userID); err != nil {
		log.Println("error while getting a user from database: ", err)
		http.Error(w, jsonError("Внутренняя ошибка сервера"), http.StatusInternalServerError)
		return
	}

	expirationTime := time.Now().Add(5 * time.Hour)

	claims := &claims{
		UserID:  userID,
		IsAdmin: u.IsAdmin,
		Claims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	// Declare the token with the algorithm used for signing, and the claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Create the JWT string
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		log.Println("error while creating a token: ", err)
		http.Error(w, jsonError("Внутренняя ошибка сервера"), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(tokenString))
}

func getUserFromRequest(r *http.Request) (*user, error) {
	tokenString := r.Header.Get("Authorization")

	claims := &claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("token is invalid")
	}

	return &user{
		ID:      claims.UserID,
		IsAdmin: claims.IsAdmin,
	}, nil
}

func jsonError(message string) string {
	errResponse, _ := json.Marshal(errorResponse{Error: message})
	return string(errResponse)
}
