package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"chat-gobelin-api/internal/users"
	"chat-gobelin-api/internal/channels"
	"time"

	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"
)

type userHandler struct {
	userRepo users.Repo
	channelRepo channels.Repo
	general channels.Channel
}

func NewUserHandler(s users.Repo, c channels.Repo, channel channels.Channel) (*userHandler, error) {
	return &userHandler{
		userRepo: s,
		channelRepo: c,
		general: channel,
	}, nil
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UserPresentation struct {
	Username string `json:"username"`
	Channels []channels.Channel `json:"channels"`
	ID       int64  `json:"id"`
}

var JWTKey = []byte("this_is_the_goblin_key_something_impossible_to_deviner")

type Claims struct {
	Username string `json:"username"`
	ID int64 `json:"id"`
	jwt.StandardClaims
}

func (h userHandler) Login(writer http.ResponseWriter, request *http.Request) {
	decoder := json.NewDecoder(request.Body)

	var req loginRequest
	err := decoder.Decode(&req)

	if err != nil {
		log.Printf("Error decoding request: %s", err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	user, err := h.userRepo.GetByUserName(request.Context(), req.Username)
	if err != nil {
		log.Printf("User retrieval issue: %s", err)
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	err = bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(req.Password))
	if err != nil {
		log.Printf("Wrong Password: %s", err)
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	expirationTime := time.Now().Add(3 * 24 * 60 * time.Minute)
	claims := &Claims{
		Username: user.Username,
		ID: user.ID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(JWTKey)
	if err != nil {
		log.Printf("Error generating JWT token %s", err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte(tokenString))

	log.Printf("User %s succesfully logged in", req.Username)
	log.Printf("Generated JWT Token: %s", tokenString)
}

func (h userHandler) Me(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()

	userRaw := ctx.Value("user")
	user := userRaw.(*users.User)
	if user == nil {
		log.Printf("No user found")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	channels, err := h.channelRepo.GetByUser(ctx, *user)
	if err != nil {
		log.Printf("Error retrieving channels: %s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	presentation := UserPresentation{
		Username: user.Username,
		ID: user.ID,
		Channels: channels,
	}
	result, err := json.Marshal(presentation)
	if err != nil {
		log.Printf("Error marshaling value: %s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusOK)
	writer.Write(result)
}

func (h userHandler) Create(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	decoder := json.NewDecoder(request.Body)

	var req loginRequest
	err := decoder.Decode(&req)

	if err != nil {
		log.Printf("Error decoding request: %s", err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	//minimum cost for now
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 4)
	if err != nil {
		log.Printf("Error hashing password: %s", err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	user, err := h.userRepo.Create(ctx, req.Username, json.RawMessage([]byte("{}")), hash)
	if err != nil {
		log.Printf("Error creating user: %s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = h.channelRepo.AddUserToChannel(ctx, h.general, *user)
	if err != nil {
		log.Printf("Error adding user to general channel: %s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	result, err := json.Marshal(user)
	if err != nil {
		log.Printf("Error marshaling value: %s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusOK)
	writer.Write(result)
}
