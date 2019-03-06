package parser

import (
	"io"
	"strings"

	"golang.org/x/net/html"
)

// GetUrls Returns all links in html document; httpOnly flag restricts to only http links
func GetUrls(body io.Reader, httpOnly bool) (urls []string) {
	doc := html.NewTokenizer(body)
	urls = []string{}

	for {
		iToken := doc.Next()
		switch iToken {
		case html.ErrorToken:
			return
		case html.StartTagToken:
			token := doc.Token()
			if token.Data == "a" {
				ok, url := getHref(token)
				if !ok {
					continue
				}

				if httpOnly && strings.Index(url, "http") == 0 {
					urls = append(urls, url)
				} else {
					urls = append(urls, url)
				}
			}
		}
	}
}

func getHref(t html.Token) (ok bool, href string) {
	for _, a := range t.Attr {
		if a.Key == "href" {
			href = a.Val
			ok = true
		}
	}

	return
}
