package handlers

import (
	"context"
	"log"
	"net/http"
	"os"

	"golang.org/x/oauth2"
)

// Scopes: OAuth 2.0 scopes provide a way to limit the amount of access that is granted to an access token.
var (
	clioOAuthConfig = &oauth2.Config{
		RedirectURL:  os.Getenv("CLIO_REDIRECT"),
		ClientID:     "ZrxvRba6rxX2Sb7PjPFNBWtXZHiH8Ckb65NiJaBM",
		ClientSecret: "It's Secret",
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

	user, err := checkSession(r)
	if err != nil {
		log.Printf("Error retrieving user information: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	token, err := clioOAuthConfig.Exchange(context.Background(), r.FormValue("code"))
	if err != nil {
		log.Printf("Token exchange failed %v\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	_, err = firestoreClient.Collection("users").Doc(user.UID).Collection("tokens").Doc("clio").Set(r.Context(), token)
	if err != nil {
		log.Printf("Failed to write token to Firestore: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile", http.StatusTemporaryRedirect)
}

func oauthClioLogout(w http.ResponseWriter, r *http.Request) {
	user, err := checkSession(r)
	if err != nil {
		log.Printf("error retrieving user information: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = firestoreClient.Collection("users").Doc(user.UID).Collection("tokens").Doc("clio").Delete(r.Context())
	if err != nil {
		log.Printf("Error deleting Clio token: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
}
