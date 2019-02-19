package handlers

import (
	"context"
	"log"
	"net/http"

	"golang.org/x/oauth2"
)

// Scopes: OAuth 2.0 scopes provide a way to limit the amount of access that is granted to an access token.
var (
	authPrompt        = oauth2.SetAuthURLParam("prompt", "consent")
	googleOAuthConfig = &oauth2.Config{
		RedirectURL:  "https://localhost:8000/auth/google/callback",
		ClientID:     "502779193562-js5kgt2vh1ko1lvov83vpq72o35adfc5.apps.googleusercontent.com",
		ClientSecret: "AGafwqjdLhcPx4SJGQcyb-j_",
		Scopes:       []string{"https://www.googleapis.com/auth/gmail.labels", "https://www.googleapis.com/auth/gmail.modify"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: "https://www.googleapis.com/oauth2/v4/token",
		},
	}
)

func oauthGoogleLogin(w http.ResponseWriter, r *http.Request) {

	// Create oauthState cookie
	oauthState := generateStateOauthCookie(w)

	/*
	   AuthCodeURL receive state that is a token to protect the user from CSRF attacks. You must always provide a non-empty string and
	   validate that it matches the the state query parameter on your redirect callback.
	*/

	u := googleOAuthConfig.AuthCodeURL(oauthState, oauth2.AccessTypeOffline, authPrompt)
	log.Println(u)
	http.Redirect(w, r, u, http.StatusTemporaryRedirect)
}

func oauthGoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Read oauthState from Cookie
	oauthState, _ := r.Cookie("oauthstate")
	if r.FormValue("state") != oauthState.Value {
		log.Println("invalid oauth google state")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

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

	// We then get the name of the user from our cache, where we set the session token
	cache := pool.Get()
	defer cache.Close()
	response, err := cache.Do("GET", sessionToken)
	if err != nil {
		// If there is an error fetching from cache, return an internal server error status
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if response == nil {
		// If the session token is not present in cache, return an unauthorized error
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	user, err := Auth.GetUser(r.Context(), string(response.([]byte)))
	if err != nil {
		log.Fatalf("error retrieving user information: %v\n", err)
		return
	}

	token, err := googleOAuthConfig.Exchange(context.Background(), r.FormValue("code"))
	if err != nil {
		log.Println("token exchange failed")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	firestoreClient.Collection("users").Doc(user.UID).Collection("tokens").Doc("google").Set(r.Context(), token)

	http.Redirect(w, r, "/profile", http.StatusTemporaryRedirect)
}
