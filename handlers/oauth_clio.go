package handlers

import (
	"context"
	"log"
	"net/http"

	"golang.org/x/oauth2"
)

// Scopes: OAuth 2.0 scopes provide a way to limit the amount of access that is granted to an access token.
var (
	clioOAuthConfig = &oauth2.Config{
		RedirectURL:  "https://localhost:8000/auth/clio/callback",
		ClientID:     "ZrxvRba6rxX2Sb7PjPFNBWtXZHiH8Ckb65NiJaBM",
		ClientSecret: "YdrMhGZRXXtlxif9PVlTYrohlXfpsCRzpjV0eLw4",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://app.clio.com/oauth/authorize",
			TokenURL: "https://app.clio.com/oauth/token",
		},
	}
)

func oauthClioLogin(w http.ResponseWriter, r *http.Request) {

	// Create oauthState cookie
	oauthState := generateStateOauthCookie(w)

	/*
	   AuthCodeURL receive state that is a token to protect the user from CSRF attacks. You must always provide a non-empty string and
	   validate that it matches the the state query parameter on your redirect callback.
	*/
	u := clioOAuthConfig.AuthCodeURL(oauthState)
	http.Redirect(w, r, u, http.StatusTemporaryRedirect)
}

func oauthClioCallback(w http.ResponseWriter, r *http.Request) {
	// Read oauthState from Cookie
	oauthState, _ := r.Cookie("oauthstate")
	if r.FormValue("state") != oauthState.Value {
		log.Println("invalid oauth clio state")
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

	token, err := clioOAuthConfig.Exchange(context.Background(), r.FormValue("code"))
	if err != nil {
		log.Println("token exchange failed")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	firestoreClient.Collection("users").Doc(user.UID).Collection("tokens").Doc("clio").Set(r.Context(), token)

	http.Redirect(w, r, "/profile", http.StatusTemporaryRedirect)
}
