package handlers

import (
	"io/ioutil"
	"net/http"
	"net/url"
)

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// Redirects back to root if URL misrouted
	if r.URL.Path != "/upload" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	q := r.URL.Query()
	link, err := url.ParseRequestURI(q.Get("link"))
	if err != nil {
		http.Error(w, "Link missing or invalid. "+err.Error(), http.StatusBadRequest)
		return
	}

	client := &http.Client{}

	resp, err := client.Get(link.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	body, err := ioutil.ReadAll(resp.Body)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(body))
}

func pushHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/push" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

}
