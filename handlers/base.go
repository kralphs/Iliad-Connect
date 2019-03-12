package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/csrf"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
)

var (
	// App is a firebase app configuration
	App *firebase.App
	// Auth is an authorization client for firebase
	Auth            *auth.Client
	firestoreClient *firestore.Client
	pool            *redis.Pool
)

// FileSystem custom file system handler
type FileSystem struct {
	fs http.FileSystem
}

func newPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:   80,
		MaxActive: 12000, // max number of connections
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", os.Getenv("REDIS_ENDPOINT"), redis.DialPassword(os.Getenv("REDIS_KEY")))
			if err != nil {
				panic(err.Error())
			}
			return c, err
		},
	}

}

// Open opens file
func (fs FileSystem) Open(path string) (http.File, error) {
	f, err := fs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if s.IsDir() {
		index := strings.TrimSuffix(path, "/") + "/index.html"
		if _, err := fs.fs.Open(index); err != nil {
			return nil, err
		}
	}

	return f, nil
}

// New returns a new http handler
func New() http.Handler {
	CSRF := csrf.Protect([]byte("32-byte-long-auth-key"))

	mux := http.NewServeMux()

	mux.HandleFunc("/profile", profileHandler)
	mux.Handle("/sessionLogin", CSRF(http.HandlerFunc(sessionHandler)))

	// oauth_Clio
	mux.HandleFunc("/auth/clio/login", oauthClioLogin)
	mux.HandleFunc("/auth/clio/callback", oauthClioCallback)
	mux.HandleFunc("/auth/clio/logout", oauthClioLogout)

	// oauth_Google
	mux.HandleFunc("/auth/google/login", oauthGoogleLogin)
	mux.HandleFunc("/auth/google/callback", oauthGoogleCallback)
	mux.HandleFunc("/auth/google/logout", oauthGoogleLogout)

	// clio_handlers
	mux.HandleFunc("/upload", uploadHandler)

	// Email handlers
	mux.HandleFunc("/email/google/push", googlePush)
	mux.HandleFunc("/email/google/watch", googleWatch)

	// Stripe Handlers
	mux.HandleFunc("/subscribe", subscribeHandler)

	// Static Content
	fileServer := http.FileServer(FileSystem{http.Dir("./static")})
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	fileServer = http.FileServer(FileSystem{http.Dir("./html")})
	mux.Handle("/html/", http.StripPrefix("/html/", fileServer))

	// Root
	mux.HandleFunc("/", indexHandler)

	return mux
}

func init() {
	var err error

	ctx := context.Background()
	conf := &firebase.Config{ProjectID: "iliad-connect-227218"}

	log.Println("Loading Firebase App")
	App, err = firebase.NewApp(ctx, conf)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("Loading Firebase Auth")
	Auth, err = App.Auth(ctx)

	log.Println("Loading Firebase Client")
	client, err := App.Firestore(ctx)
	if err != nil {
		log.Fatalln("Error opening firestore Client: " + err.Error())
	}
	firestoreClient = client
	log.Println("Loading New Redis Connection Pool")
	pool = newPool()
	log.Println("Initialize Complete")
}

func generateStateOauthCookie(w http.ResponseWriter) string {
	var expiration = time.Now().Add(365 * 24 * time.Hour)

	b := make([]byte, 16)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)
	cookie := http.Cookie{Name: "oauthstate", Value: state, Expires: expiration}
	http.SetCookie(w, &cookie)

	return state
}

func sessionID() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return base64.URLEncoding.EncodeToString(b), err
}

func checkSession(r *http.Request) (*auth.UserRecord, error) {
	c, err := r.Cookie("session_token")
	if err != nil {
		if err == http.ErrNoCookie {
			return nil, err
		}
		// For any other type of error, return a bad request status
		return nil, errors.New("Bad Request")
	}
	sessionToken := c.Value

	cache := pool.Get()
	defer cache.Close()
	// We then get the name of the user from our cache, where we set the session token
	response, err := cache.Do("GET", sessionToken)
	if err != nil {
		// If there is an error fetching from cache, return an internal server error status
		return nil, err
	}

	if response == nil {
		return nil, errors.New("Unauthorized")
	}

	user, err := Auth.GetUser(r.Context(), string(response.([]byte)))
	if err != nil {
		return nil, err
	}

	return user, nil
}
