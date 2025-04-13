package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

var (
	flagRoot   string
	flagListen string
)

func main() {
	flag.StringVar(&flagRoot, "root", ".", "Directory to serve files from")
	flag.StringVar(&flagListen, "listen", ":8000", "Address to listen on ")
	flag.Parse()

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(&url.URL{
				Scheme: "http",
				Host:   "appldnld.apple.com",
			})
		},
	}
	http.Handle("/ipod/", proxy)
	http.Handle("/iPod/", proxy)
	http.Handle("/", http.FileServer(http.Dir(flagRoot)))

	log.Fatal(http.ListenAndServe(flagListen, nil))
}
