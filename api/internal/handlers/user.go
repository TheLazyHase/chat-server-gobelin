package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"chat-gobelin-api/internal"
	"chat-gobelin-api/internal/users"
	"chat-gobelin-api/internal/channels"
	"chat-gobelin-api/internal/websockets"

	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"
)

type userHandler struct {
	hub *websockets.BroadcastHub
	userRepo users.Repo
	channelRepo channels.Repo
	general channels.Channel
}

func NewUserHandler(h *websockets.BroadcastHub, s users.Repo, c channels.Repo, channel channels.Channel) (*userHandler, error) {
	return &userHandler{
		hub: h,
		userRepo: s,
		channelRepo: c,
		general: channel,
	}, nil
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type userCreationRequest struct {
	Username string           `json:"username" binding:"required"`
	Password string           `json:"password" binding:"required"`
	Metadata *json.RawMessage `json:"metadata"`
}

type userEditionRequest struct {
	Metadata json.RawMessage `json:"metadata" binding:"required"`
}

type UserPresentation struct {
	Username string             `json:"username"`
	Channels []channels.Channel `json:"channels"`
	ID       int64              `json:"id"`
	Metadata *json.RawMessage   `json:"metadata"`
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


func (h userHandler) GetAll(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()

	users, err := h.userRepo.GetAll(ctx)
	if err != nil {
		log.Printf("Error retrieving users: %s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	namespace := internal.GetNamespace(ctx)

	presentations := make([]UserPresentation, 0, len(users))
	for _, user := range users {
		empty := json.RawMessage([]byte("{}"))
		presentation := UserPresentation{
			Username: user.Username,
			ID: user.ID,
			Metadata: &empty,
		}
		if user.Metadata != nil {
			meta := user.Metadata

			if namespace != "" {
				meta = make(map[string]json.RawMessage)
				if namespaced, ok := user.Metadata[namespace]; ok {
					meta[namespace] = namespaced
				}
			}
			rawMeta, err := json.Marshal(meta)
			if err != nil {
				log.Printf("Error namespacing metadatas: %s", err.Error())
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
			jsonMeta := json.RawMessage(rawMeta)
			presentation.Metadata = &jsonMeta
		}
		presentations = append(presentations, presentation)
	}

	result, err := json.Marshal(presentations)
	if err != nil {
		log.Printf("Error marshaling value: %s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusOK)
	writer.Write(result)
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
	log.Printf("test: %v %s", user, internal.GetNamespace(ctx))
	if user.Metadata != nil {
		meta := user.Metadata
		namespace := internal.GetNamespace(ctx)
		if namespace != "" {
			meta = make(map[string]json.RawMessage)
			if namespaced, ok := user.Metadata[namespace]; ok {
				meta[namespace] = namespaced
			}
		}
		rawMeta, err := json.Marshal(meta)
		if err != nil {
			log.Printf("Error namespacing metadatas: %s", err.Error())
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		jsonMeta := json.RawMessage(rawMeta)
		presentation.Metadata = &jsonMeta
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

	var req userCreationRequest
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

	meta := json.RawMessage([]byte("{}"))
	if req.Metadata != nil {
		meta = *req.Metadata
		namespace := internal.GetNamespace(ctx)
		if namespace == "" {
			log.Printf("Metadatas given without namespace")
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		namespacedMeta := make(map[string]json.RawMessage)
		namespacedMeta[namespace] = *req.Metadata
		meta, err = json.Marshal(namespacedMeta)
		if err != nil {
			log.Printf("Error namespacing metadatas: %s", err.Error())
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	user, err := h.userRepo.Create(ctx, req.Username, meta, hash)
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

func (h userHandler) Edit(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()

	namespace := internal.GetNamespace(ctx)
	if namespace == "" {
		log.Printf("Metadatas given without namespace")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	decoder := json.NewDecoder(request.Body)
	var req userEditionRequest
	err := decoder.Decode(&req)
	if err != nil {
		log.Printf("Error decoding request: %s", err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	userRaw := ctx.Value("user")
	user := userRaw.(*users.User)
	if user == nil {
		log.Printf("No user found")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	oldMeta := user.Metadata
	oldMeta[namespace] = req.Metadata
	meta, err := json.Marshal(oldMeta)
	if err != nil {
		log.Printf("Error namespacing metadatas: %s", err.Error())
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	user, err = h.userRepo.EditMetadata(ctx, *user, meta)
	if err != nil {
		log.Printf("Error creating user: %s", err.Error())
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
