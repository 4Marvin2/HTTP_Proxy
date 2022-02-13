package httpproxy

import (
	"fmt"
	"log"
	"net"
	"strings"
)

const (
	Host = "127.0.0.1"
	Port = "8080"
)

func Run() {
	ln, err := net.Listen("tcp", ":"+Port)
	if err != nil {
		log.Fatalf("Unable to start listener, %v\n", err)
	}
	defer ln.Close()
	log.Printf("Start listening on port %s\n", Port)
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Printf("Accept connection err: %v\n", err)
		}
		go handleConnection(c)
	}
}

func handleConnection(c net.Conn) {
	fmt.Printf("Serving %s\n", c.RemoteAddr().String())

	request, err := readResponse(c)
	if err != nil {
		log.Printf("Cannot read data from initiator: %v\n", err)
		c.Close()
		return
	}

	hostAndPort, newReq := preparationForProxying(request)

	rConn, err := net.Dial("tcp", hostAndPort)
	if err != nil {
		log.Printf("Cannot connect to remote host: %v\n", err)
		c.Close()
		return
	}
	defer rConn.Close()

	if _, err := rConn.Write(newReq); err != nil {
		log.Printf("Cannot write to remote host: %v\n", err)
		c.Close()
		rConn.Close()
		return
	}

	response, err := readResponse(rConn)
	if err != nil {
		log.Printf("Cannot read data from remote host: %v\n", err)
		c.Close()
		rConn.Close()
		return
	}
	rConn.Close()

	if _, err := c.Write(response); err != nil {
		log.Printf("Cannot write to initiator: %v\n", err)
		c.Close()
		return
	}
	c.Close()
}

func readResponse(c net.Conn) ([]byte, error) {
	var request []byte
	contentLength := 0
	for {
		buffer := make([]byte, 256)
		n, err := c.Read(buffer)
		if err != nil {
			return request, err
		}
		contentLength += n
		request = append(request, buffer...)
		if n < len(buffer) {
			request = request[:contentLength]
			break
		}
	}

	return request, nil
}

func preparationForProxying(request []byte) (string, []byte) {
	parsedReq := strings.Split(string(request), "\n")

	firstLine := strings.Split(parsedReq[0], " ")
	url := firstLine[1]

	parseURL := strings.Split(firstLine[1], "/")
	fmt.Println(parseURL)
	protocol := parseURL[0]
	hostAndPort := parseURL[2]
	if !strings.Contains(hostAndPort, ":") {
		if strings.Contains(protocol, "https") {
			hostAndPort += ":443"
		} else if strings.Contains(protocol, "http") {
			hostAndPort += ":80"
		}
	}
	iter := 3
	var path string
	for iter < len(parseURL) {
		path += "/" + parseURL[iter]
		iter++
	}

	newReq := strings.Replace(string(request), url, path, 1)

	proxyConnHeaderStartIndex := strings.Index(newReq, "Proxy-Connection")
	proxyConnHeaderEndIndex := strings.Index(newReq[proxyConnHeaderStartIndex:], "\n") + proxyConnHeaderStartIndex
	newReq = newReq[:proxyConnHeaderStartIndex] + newReq[proxyConnHeaderEndIndex+1:]

	return hostAndPort, []byte(newReq)
}
