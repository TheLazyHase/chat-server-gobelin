package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
	"os"
	"os/signal"
	"syscall"

	"chat-gobelin-api/internal/handlers"
	"chat-gobelin-api/internal/users"
	"chat-gobelin-api/internal/channels"
	"chat-gobelin-api/internal/messages"
	"chat-gobelin-api/internal/websockets"
	"chat-gobelin-api/internal"

	"github.com/gorilla/mux"
	"github.com/dgrijalva/jwt-go"
)

type authenticationMiddleware struct {
	Repo users.Repo
}

func main() {
	//TODO: deal with environments
	db, err := internal.GetDevDB()

	if err != nil {
		log.Fatal(err)
	}

	//New event broadcaster, run in background to dispatch update to websockets
	hub := websockets.NewBroadcastHub()
	go hub.Run()

	userRepo, err := users.NewRepo(db)
	if err != nil {
		log.Fatal(err)
	}

	channelRepo, err := channels.NewRepo(db)
	if err != nil {
		log.Fatal(err)
	}

	messageRepo, err := messages.NewRepo(db)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := context.WithCancel(context.Background())

	//Internal clock
	ticker := time.NewTicker(600 * time.Second)

	internalUser, err := userRepo.GetByUserName(ctx, "internal-clock")
	log.Printf("Error: %v", err)
	if err != nil {
		internalUser, err = userRepo.Create(ctx, "internal-clock",  json.RawMessage([]byte("{}")), []byte("111111111"))
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("User: %v", internalUser)

	generalChannel, _ := channelRepo.GetByName(ctx, "general")
	if generalChannel == nil {
		generalChannel, err = channelRepo.Create(ctx, "general", json.RawMessage(`{}`))
		if err != nil {
			log.Fatal(err)
		}
	}

	go func() {
		//SIGINT or SIGTERM
		var interrupt = make(chan os.Signal,2)
		signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
		<-interrupt
		stop()
	}()

	go func() {
		for {
			select {
			case <- ctx.Done():
				return
			case _ = <-ticker.C:
				messageRepo.Create(ctx, fmt.Sprintf("clock tick : %s", time.Now().Format(time.UnixDate)), json.RawMessage(`{}`), *generalChannel, *internalUser, true, nil)
			}
		}
	}()

	userHandler, err := handlers.NewUserHandler(hub, userRepo, channelRepo, *generalChannel)
	if err != nil {
		log.Fatal(err)
	}

	channelHandler, err := handlers.NewChannelHandler(hub, channelRepo, userRepo)
	if err != nil {
		log.Fatal(err)
	}

	messageHandler, err := handlers.NewMessageHandler(hub, messageRepo)
	if err != nil {
		log.Fatal(err)
	}

	amw := authenticationMiddleware{
		Repo: userRepo,
	}

	router := mux.NewRouter()
	router.Use(logRequests)
	router.Use(setAccessControlHeader)
	router.Use(amw.authMiddleware)
	router.Use(namespace)

	router.HandleFunc("/login", userHandler.Login).Methods(http.MethodPost)
	router.HandleFunc("/user", userHandler.Create).Methods(http.MethodPost)
	router.HandleFunc("/user", userHandler.Edit).Methods(http.MethodPut)
	router.HandleFunc("/users", userHandler.GetAll).Methods(http.MethodGet)
	router.HandleFunc("/me", userHandler.Me).Methods(http.MethodGet)
	router.HandleFunc("/channel/socket", channelHandler.ServeWS).Methods(http.MethodGet)
	/*router.HandleFunc("/channel", channelHandler.Get).Methods(http.MethodGet)*/
	router.HandleFunc("/channel/connected", channelHandler.GetConnectedUsers).Methods(http.MethodGet)
	router.HandleFunc("/channel", channelHandler.Create).Methods(http.MethodPost)
	router.HandleFunc("/channel", channelHandler.Edit).Methods(http.MethodPut)
	router.HandleFunc("/channel/enter", channelHandler.EnterChannel).Methods(http.MethodPost)
	router.HandleFunc("/channel/message", messageHandler.GetMessages).Methods(http.MethodGet)
	router.HandleFunc("/channel/message", messageHandler.CreateMessage).Methods(http.MethodPost)
	router.HandleFunc("/channel/message", messageHandler.EditMessage).Methods(http.MethodPut)
	router.Methods(http.MethodOptions).HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}).Methods(http.MethodOptions)

	log.Println("Application has started. Listening port is 4000")
	err = http.ListenAndServeTLS(":4000", "certificate/cert.pem", "certificate/privkey.pem", router)
	log.Fatal(err)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqDump, _ := httputil.DumpRequest(r, true)
		log.Printf("REQUEST:\n%s", string(reqDump))
		// Extract claims and pass to next handler
		next.ServeHTTP(w, r)
	})
}

func setAccessControlHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Namespace")
		next.ServeHTTP(w, r)
	})
}

func (amw *authenticationMiddleware) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//No auth required for login
		if strings.TrimPrefix(r.URL.Path, "/") == "login" && r.Method == http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		if strings.TrimPrefix(r.URL.Path, "/") == "user" && r.Method == http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		if strings.TrimPrefix(r.URL.Path, "/") == "users" && r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		//Neither for OPTION
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

		if tokenStr == "" {
			tokenStr = r.URL.Query().Get("authorization")
		}
		if tokenStr == "" {
			log.Printf("No token")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		token, err := jwt.ParseWithClaims(tokenStr, &handlers.Claims{}, func(token *jwt.Token) (interface{}, error) {
			return handlers.JWTKey, nil
		})
		if err != nil || !token.Valid {
			log.Printf("Invalid token")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		claims := token.Claims.(*handlers.Claims)
		log.Printf("token have %s and %d", claims.Username, claims.ID)
		currentUser, err := amw.Repo.GetByID(r.Context(), claims.ID)
		if err != nil  {
			log.Printf("Error retrieving user: %s", err.Error())
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), "user", currentUser)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func namespace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		namespace := r.URL.Query().Get("namespace")

		if namespace == "" {
			namespace = r.Header.Get("Namespace")
		}
		log.Printf("Namespace on request: %s", namespace)
		if namespace == "" {
			next.ServeHTTP(w, r)
			return
		}

		ns := internal.HaveNamespace{
			NameSpace: namespace,
		}

		ctx := context.WithValue(r.Context(), "namespace", &ns)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
