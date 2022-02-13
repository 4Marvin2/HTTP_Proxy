package main

import (
	"proxy/httpProxy"
	"runtime"
)

const (
	Host = "127.0.0.1"
	Port = "8080"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	httpProxy.Run()
}
