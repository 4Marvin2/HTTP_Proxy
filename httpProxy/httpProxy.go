package httpProxy

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	Host = "127.0.0.1"
	Port = "8080"
)

const BufferSuze = 4096
const (
	SlashRCode = 13
	SlashNCode = 10
)

func Run() {
	// ln, err := net.Listen("tcp", ":"+Port)
	// if err != nil {
	// 	log.Fatalf("Unable to start listener, %v\n", err)
	// }
	// defer ln.Close()
	// log.Printf("Start listening on port %s\n", Port)
	// for {
	// 	c, err := ln.Accept()
	// 	if err != nil {
	// 		log.Printf("Accept connection err: %v\n", err)
	// 	}
	// 	go handleConnection(c)
	// }

	cer, err := tls.LoadX509KeyPair("certs/127.0.0.1.crt", "cert.key")
	if err != nil {
		log.Println(err)
		return
	}

	config := &tls.Config{
		Certificates:       []tls.Certificate{cer},
		InsecureSkipVerify: true,
	}
	ln, err := tls.Listen("tcp", ":8080", config)
	if err != nil {
		log.Println(err)
		return
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(c net.Conn) {
	log.Printf("Serving %s\n", c.RemoteAddr().String())

	request, err := readResponse(c)
	if err != nil {
		log.Printf("Cannot read data from initiator: %v\n", err)
		c.Close()
		return
	}
	fmt.Println(string(request))

	parsedReq := strings.Split(string(request), "\n")
	firstLine := strings.Split(parsedReq[0], " ")
	method := firstLine[0]
	url := firstLine[1]
	if method == "CONNECT" {
		parsedUrl := strings.Split(url, ":")
		err := handleSecureConn(c, parsedUrl[0])
		if err != nil {
			log.Printf("Cannot establishe secure connection: %v", err)
			c.Close()
			return
		}
		request, err = readResponse(c)
		if err != nil {
			log.Printf("Cannot read data from initiator after established secure conn: %v\n", err)
			c.Close()
			return
		}
		fmt.Println(string(request))
		// newReq := strings.Replace(string(request), "1.1", "2", 1)
		// fmt.Println(newReq)
		cer, err := tls.LoadX509KeyPair(parsedUrl[0]+".crt", "cert.key")
		rConn, err := tls.Dial("tcp", url, &tls.Config{Certificates: []tls.Certificate{cer}})
		err = rConn.Handshake()
		fmt.Println(err)
		// rConn, err := net.Dial("tcp", "mail.ru:443")
		// request := "GET / HTTP/1.1\r\nHost: mail.ru\r\nUser-Agent: Go-Proxy\r\nAccept: */*\r\n\r\n"
		if _, err := rConn.Write([]byte(request)); err != nil {
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
		fmt.Println(string(response))

		fmt.Println(4)
		if _, err := c.Write(response); err != nil {
			log.Printf("Cannot write to initiator: %v\n", err)
			c.Close()
			return
		}
		fmt.Println(err)
		c.Close()
		return
	}

	hostAndPort, newReq := preparationForProxying(c, request)

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

	time.Sleep(100 * time.Millisecond)

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
	var err error
	var n int
	for err != io.EOF {
		buffer := make([]byte, BufferSuze)
		n, err = c.Read(buffer)
		if err != nil && err != io.EOF {
			return request, err
		}
		contentLength += n
		request = append(request, buffer...)
		request = request[:contentLength]
		if n < BufferSuze {
			break
		}
	}

	return request, nil
}

func preparationForProxying(c net.Conn, request []byte) (string, []byte) {
	parsedReq := strings.Split(string(request), "\n")
	firstLine := strings.Split(parsedReq[0], " ")
	url := firstLine[1]

	parseURL := strings.Split(firstLine[1], "/")
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

func handleSecureConn(c net.Conn, host string) error {
	cmd := exec.Command("/bin/sh", "./scripts/gen_cert.sh", host, strconv.Itoa(rand.Intn(math.MaxInt32)))

	err := cmd.Start()
	if err != nil {
		return errors.New(fmt.Sprintf("Start create cert file script error: %v\n", err))
	}

	err = cmd.Wait()
	if err != nil {
		return errors.New(fmt.Sprintf("Wait create cert file script error: %v\n", err))
	}

	resp := "HTTP/1.0 200 Connection established\r\n\r\n" +
		"Proxy-agent: Golang-Proxy\r\n\r\n\r\n"
	if _, err := c.Write([]byte(resp)); err != nil {
		return errors.New(fmt.Sprintf("Cannot write connect response to initiator: %v\n", err))
	}

	return nil
}
