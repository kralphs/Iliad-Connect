package handlers

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/csrf"
	"google.golang.org/api/iterator"
	//	firebase "firebase.google.com/go"
)

type templateParams struct {
	Name         string
	Photo        string
	Clio         bool
	Email        bool
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
	user, err := checkSession(r)
	if err != nil {
		switch err.Error() {
		case "Unauthorized":
			w.WriteHeader(http.StatusUnauthorized)
			return
		case "Bad Request":
			w.WriteHeader(http.StatusBadRequest)
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	params.Name = user.DisplayName
	params.Photo = user.PhotoURL

	userRef := firestoreClient.Collection("users").Doc(user.UID)
	userDoc, err := userRef.Get(r.Context())
	if err != nil {
		log.Printf("error retrieving user information: %v\n", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Checks for Oauth tokens stored against the user
	tokens := userRef.Collection("tokens").DocumentRefs(r.Context())
	for {
		token, err := tokens.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("Error checking tokens: %v\n", err)
		}
		switch token.ID {
		case "email":
			params.Email = true
		case "clio":
			params.Clio = true
		}
	}

	// Temporarily restricting access to users with the beta flag set to true
	if userDoc.Data()["beta"] == true {
		profileTemplate.Execute(w, params)
		return
	}

	w.WriteHeader(http.StatusUnauthorized)

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
			log.Printf("error verifying ID token: %v\n", err)
			return
		}

		sessionToken, err := sessionID()
		if err != nil {
			log.Printf("error creating session token: %v\n", err)
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
