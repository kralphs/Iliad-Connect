package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"

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
			//			reply, err := c.Do("AUTH", os.Getenv("REDIS_KEY"))
			//			if err != nil {
			//				c.Close()
			//				return nil, err
			//			}
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

	mux := http.NewServeMux()

	mux.HandleFunc("/profile", profileHandler)
	mux.HandleFunc("/sessionLogin", sessionHandler)

	// oauth_Clio
	mux.HandleFunc("/auth/clio/login", oauthClioLogin)
	mux.HandleFunc("/auth/clio/callback", oauthClioCallback)

	// oauth_Google
	mux.HandleFunc("/auth/google/login", oauthGoogleLogin)
	mux.HandleFunc("/auth/google/callback", oauthGoogleCallback)

	// clio_handlers
	mux.HandleFunc("/upload", uploadHandler)
	mux.HandleFunc("/push", pushHandler)

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

func sessionID() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return base64.URLEncoding.EncodeToString(b), err
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