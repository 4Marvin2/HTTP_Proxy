package main

import (
	"log"
	"net/http"
	"proxy/httpProxy"
	"proxy/repeater"
	"runtime"
)

const (
	Host = "127.0.0.1"
	Port = "8080"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	p := httpProxy.Init()

	go p.Run()

	r := repeater.Init(p)

	apiServer := http.Server{
		Addr:    ":8000",
		Handler: r,
	}

	err := apiServer.ListenAndServe()

	if err != nil {
		log.Fatalf("Failed repeater run: %v", err)
	}
}
