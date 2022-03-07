package httpProxy

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jackc/pgx"
)

const (
	Host = "127.0.0.1"
	Port = ":8080"
)

const (
	Username = "marvin"
	Password = "vbif"
	DBName   = "http_proxy"
	DBHost   = "127.0.0.1"
	DBPort   = "5432"
)

type Proxy struct {
	db *pgx.ConnPool
}

func Init() *Proxy {
	ConnStr := fmt.Sprintf("user=%s dbname=%s password=%s host=%s port=%s sslmode=disable",
		Username,
		DBName,
		Password,
		DBHost,
		DBPort)

	pgxConnectionConfig, err := pgx.ParseConnectionString(ConnStr)
	if err != nil {
		log.Fatalf("Invalid config string: %s", err)
	}

	pool, err := pgx.NewConnPool(pgx.ConnPoolConfig{
		ConnConfig:     pgxConnectionConfig,
		MaxConnections: 100,
		AfterConnect:   nil,
		AcquireTimeout: 0,
	})
	if err != nil {
		log.Fatalf("Error %s occurred during connection to database", err)
	}

	return &Proxy{db: pool}
}

func (p Proxy) Run() {
	server := http.Server{
		Addr: Port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				p.HandleHTTPS(w, r)
			} else {
				p.HandleHTTP(w, r)
			}
		}),
	}

	log.Fatal(server.ListenAndServe())
}

func (p Proxy) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	for key := range r.Header {
		if key == "Proxy-Connection" {
			r.Header.Del(key)
		}
	}

	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = p.saveToDB(r, resp)
	if err != nil {
		log.Printf("fail save to db: %v", err)
	}
}

func (p Proxy) HandleHTTPS(w http.ResponseWriter, r *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Println("Hijacking not supported")
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	localConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Printf("hijacking error: %v", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}

	_, err = localConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	if err != nil {
		log.Printf("handshaking failed: %v", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		localConn.Close()
		return
	}
	defer localConn.Close()

	host := strings.Split(r.Host, ":")[0]

	tlsConfig, err := p.generateTLSConfig(host, r.URL.Scheme)
	if err != nil {
		log.Printf("error getting cert: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tlsLocalConn := tls.Server(localConn, &tlsConfig)
	err = tlsLocalConn.Handshake()
	if err != nil {
		tlsLocalConn.Close()
		log.Printf("handshaking failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tlsLocalConn.Close()

	remoteConn, err := tls.Dial("tcp", r.URL.Host, &tlsConfig)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer remoteConn.Close()

	reader := bufio.NewReader(tlsLocalConn)
	request, err := http.ReadRequest(reader)
	if err != nil {
		log.Printf("error getting request: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	requestByte, err := httputil.DumpRequest(request, true)
	if err != nil {
		log.Printf("failed to dump request: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = remoteConn.Write(requestByte)
	if err != nil {
		log.Printf("failed to write request: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	serverReader := bufio.NewReader(remoteConn)
	response, err := http.ReadResponse(serverReader, request)
	if err != nil {
		log.Printf("failed to read response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rawResponse, err := httputil.DumpResponse(response, true)
	if err != nil {
		log.Printf("failed to dump response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = tlsLocalConn.Write(rawResponse)
	if err != nil {
		log.Printf("fail to write response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	request.URL.Scheme = "https"
	hostAndPort := strings.Split(r.URL.Host, ":")
	request.URL.Host = hostAndPort[0]
	err = p.saveToDB(request, response)
	if err != nil {
		log.Printf("fail save to db: %v", err)
	}
}

func (p Proxy) generateTLSConfig(host string, URL string) (tls.Config, error) {
	cmd := exec.Command("/bin/sh", "./scripts/gen_cert.sh", host, strconv.Itoa(rand.Intn(math.MaxInt32)))

	err := cmd.Start()
	if err != nil {
		return tls.Config{}, errors.New(fmt.Sprintf("Start create cert file script error: %v\n", err))
	}

	err = cmd.Wait()
	if err != nil {
		return tls.Config{}, errors.New(fmt.Sprintf("Wait create cert file script error: %v\n", err))
	}

	tlsCert, err := tls.LoadX509KeyPair("certs/"+host+".crt", "cert.key")
	if err != nil {
		log.Println("error loading pair", err)
		return tls.Config{}, err
	}

	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ServerName:   URL,
	}

	return tlsConfig, nil
}

func (p Proxy) saveToDB(req *http.Request, resp *http.Response) error {
	insertReqQuery := `INSERT INTO request (method, scheme, host, path, header, body)
	values ($1, $2, $3, $4, $5, $6) RETURNING id`
	var reqId int32
	reqHeaders, err := json.Marshal(req.Header)
	if err != nil {
		return err
	}
	fmt.Println(1)
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	fmt.Println(2)
	err = p.db.QueryRow(insertReqQuery, req.Method, req.URL.Scheme, req.URL.Host, req.URL.Path, reqHeaders, string(reqBody)).Scan(&reqId)
	if err != nil {
		return err
	}

	insertRespQuery := `INSERT INTO response (req_id, code, resp_message, header, body)
	values ($1, $2, $3, $4, $5) RETURNING id`
	var respId int32
	respHeaders, err := json.Marshal(req.Header)
	if err != nil {
		return err
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = p.db.QueryRow(insertRespQuery, reqId, resp.StatusCode, resp.Status[4:], respHeaders, respBody).Scan(&respId)
	if err != nil {
		return err
	}

	return nil
}
