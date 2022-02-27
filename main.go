package main

import (
	"log"
	"proxy/httpProxy"
	"runtime"
)

const (
	Host = "127.0.0.1"
	Port = "8080"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	p, err := httpProxy.Init()
	if err != nil {
		log.Fatalf("Init proxy error: %v", err)
		return
	}

	p.Run()
}
