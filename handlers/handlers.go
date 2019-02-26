package handlers

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/csrf"
	//	firebase "firebase.google.com/go"
)

type templateParams struct {
	Name         string
	Photo        string
	Clio         bool
	Google       bool
	Subscription bool
}

var (
	publishableKey    = os.Getenv("PUBLISHABLE_KEY")
	indexTemplate     = template.Must(template.ParseFiles("index.html"))
	profileTemplate   = template.Must(template.ParseFiles("html/profile/index.html"))
	subscribeTemplate = template.Must(template.ParseFiles("subscribe.html"))

	params templateParams
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling Index")
	//	indexTemplate.Execute(w, map[string]interface{}{
	//		csrf.TemplateTag: csrf.TemplateField(r),
	//	})
	http.ServeFile(w, r, "html/landing/index.html")
	return
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
	// We can obtain the session token from the requests cookies, which come with every request
	c, err := r.Cookie("session_token")
	if err != nil {
		if err == http.ErrNoCookie {
			// If the cookie is not set, return an unauthorized status
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// For any other type of error, return a bad request status
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	sessionToken := c.Value
	log.Println(sessionToken)

	cache := pool.Get()
	defer cache.Close()
	// We then get the name of the user from our cache, where we set the session token
	response, err := cache.Do("GET", sessionToken)
	if err != nil {
		// If there is an error fetching from cache, return an internal server error status
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Println("got the token")
	if response == nil {
		// If the session token is not present in cache, return an unauthorized error
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	user, err := Auth.GetUser(r.Context(), string(response.([]byte)))
	if err != nil {
		log.Fatalf("error retrieving user account: %v\n", err)
		return
	}

	params.Name = user.DisplayName
	params.Photo = user.PhotoURL

	doc, err := firestoreClient.Collection("users").Doc(user.UID).Get(r.Context())
	if err != nil {
		log.Fatalf("error retrieving user information: %v\n", err)
		return
	}
	log.Println(doc.Data())

	profileTemplate.Execute(w, params)
	return
}

func sessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		log.Println("Preparing Session")
		w.Header().Set("X-CSRF-Token", csrf.Token(r))
		return
	}

	if r.Method == http.MethodPost {
		log.Println("Initiating Session")
		ctx := r.Context()
		idToken := r.FormValue("token")

		token, err := Auth.VerifyIDToken(ctx, idToken)
		if err != nil {
			log.Fatalf("error verifying ID token: %v\n", err)
			return
		}

		sessionToken, err := sessionID()
		if err != nil {
			log.Fatalf("error creating session token: %v\n", err)
			return
		}
		log.Println("Token Created")
		// Set the token in the cache, along with the user whom it represents
		// The token has an expiry time of 5 days
		cache := pool.Get()
		defer cache.Close()
		_, err = cache.Do("SETEX", sessionToken, "432000", token.UID)
		if err != nil {
			// If there is an error in setting the cache, return an internal server error
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		log.Println("sessionToken sent to cache")
		// Finally, we set the client cookie for "session_token" as the session token we just generated
		// we also set an expiry time of 120 seconds, the same as the cache
		log.Println("session cookie sent")
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    sessionToken,
			Expires:  time.Now().Add(432000 * time.Second),
			Secure:   true,
			HttpOnly: true,
		})

	}
}

func subscribeHandler(w http.ResponseWriter, r *http.Request) {
	subscribeTemplate.Execute(w, nil)
	return
}
