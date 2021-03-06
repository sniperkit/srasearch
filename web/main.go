package web

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/hypebeast/gojistaticbin"
	"github.com/zenazn/goji"
	"github.com/zenazn/goji/bind"
	"github.com/zenazn/goji/graceful"

	"github.com/indraniel/srasearch/assets"
	"github.com/indraniel/srasearch/searchdb"
	"github.com/indraniel/srasearch/web/controllers"
	"github.com/indraniel/srasearch/web/routes"
)

type Web struct {
	Debug     bool
	IndexPath string
	Host      string
	Port      int
}

func NewWeb(port int, host, idxPath string, debug bool) *Web {
	return &Web{
		Debug:     debug,
		IndexPath: idxPath,
		Host:      host,
		Port:      port,
	}
}

func (w Web) Main() {
	// Setup Search Database
	log.Println(
		"Loading & Initializing the Bleve Search Database:",
		w.IndexPath,
		"-- Please wait...",
	)

	if err := searchdb.Init(w.IndexPath); err != nil {
		log.Fatalf(
			"Couldn't open Bleve Index Path: %s : %s",
			w.IndexPath,
			err,
		)
	}

	log.Println("Done")

	// Static files setup
	goji.Use(
		gojistaticbin.Staticbin(
			"web/static",
			assets.Asset,
			gojistaticbin.Options{
				SkipLogging: false,
				IndexFile:   "",
			},
		),
	)

	// Add routes
	routes.Include()

	// Controller Settings
	controllers.Init(w.Debug, w.IndexPath)

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
