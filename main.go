package main

import (
	"proxy/httpproxy"
	"runtime"
)

const (
	Host = "127.0.0.1"
	Port = "8080"
)

func main() {
	// Просим Go использовать все имеющиеся в системе процессоры.
	runtime.GOMAXPROCS(runtime.NumCPU())

	httpproxy.Run()
}
