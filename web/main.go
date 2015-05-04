package web

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/hypebeast/gojistatic"
	"github.com/zenazn/goji"
	"github.com/zenazn/goji/bind"
	"github.com/zenazn/goji/graceful"

	"github.com/indraniel/srasearch/searchdb"
	"github.com/indraniel/srasearch/web/routes"
)

type Web struct {
	IndexPath string
	Host      string
	Port      int
}

func NewWeb(port int, host, idxPath string) *Web {
	return &Web{
		IndexPath: idxPath,
		Host:      host,
		Port:      port,
	}
}

func (w Web) Main() {
	// Setup Search Database
	if err := searchdb.Init(w.IndexPath); err != nil {
		log.Fatalf(
			"Couldn't open Bleve Index Path: %s : %s",
			w.IndexPath,
			err,
		)
	}

	// Static files setup
	goji.Use(
		gojistatic.Static(
			"web/static",
			gojistatic.StaticOptions{SkipLogging: false},
		),
	)

	// Add routes
	routes.Include()

	// Run Goji
	w.Serve()
}

func (w Web) Serve() {
	http.Handle("/", goji.DefaultMux)

	address := fmt.Sprintf("%s:%d", w.Host, w.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalln("Trouble starting up listener on ", address)
	}

	log.Println("Starting Goji on", listener.Addr())
	log.Println("Bleve IndexPath is:", w.IndexPath)

	// taken from the goji internals
	graceful.HandleSignals()
	bind.Ready()

	graceful.PreHook(func() { log.Printf("Goji received signal, gracefully stopping") })
	graceful.PostHook(func() { log.Printf("Goji stopped") })

	// start up the server
	err = graceful.Serve(listener, http.DefaultServeMux)
	if err != nil {
		log.Fatal(err)
	}

	graceful.Wait()
}
