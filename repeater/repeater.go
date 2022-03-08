package repeater

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"proxy/httpProxy"
	"proxy/repeater/models"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx"
)

type Api struct {
	db     *pgx.ConnPool
	proxy  *httpProxy.Proxy
	router *mux.Router
}

const (
	Host = "127.0.0.1"
	Port = ":8000"
)

const (
	Username = "marvin"
	Password = "vbif"
	DBName   = "http_proxy"
	DBHost   = "127.0.0.1"
	DBPort   = "5432"
)

func Init(proxy *httpProxy.Proxy) *Api {
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

	router := mux.NewRouter()

	api := &Api{
		db:     pool,
		proxy:  proxy,
		router: router,
	}

	router.HandleFunc("/repeat/{request_id:[0-9]+}", api.RepeatRequest)
	router.HandleFunc("/requests/{request_id:[0-9]+}", api.GetRequest)
	router.HandleFunc("/requests", api.GetAllRequests)
	router.HandleFunc("/scan/{request_id:[0-9]+}", api.ScanRequest)

	return api
}

func (api *Api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.router.ServeHTTP(w, r)
}

func (api *Api) RepeatRequest(w http.ResponseWriter, r *http.Request) {
	requestId, err := strconv.Atoi(mux.Vars(r)["request_id"])
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	query := `select id, method, scheme, host, path, header, body from request where id = $1`

	savedReq := &models.Request{}
	byteHeaders := make([]byte, 0)

	err = api.db.QueryRow(query, requestId).Scan(&savedReq.Id, &savedReq.Method, &savedReq.Scheme, &savedReq.Host, &savedReq.Path, &byteHeaders, &savedReq.Body)
	if err != nil {
		return
	}
	var jsonHeaders http.Header
	err = json.Unmarshal(byteHeaders, &jsonHeaders)

	req := &http.Request{
		Method: savedReq.Method,
		URL: &url.URL{
			Scheme: savedReq.Scheme,
			Host:   savedReq.Host,
			Path:   savedReq.Path,
		},
		Body:   ioutil.NopCloser(strings.NewReader(savedReq.Body)),
		Host:   savedReq.Host,
		Header: jsonHeaders,
	}

	api.proxy.HandleHTTP(w, req, false)
}

func (api *Api) GetRequest(w http.ResponseWriter, r *http.Request) {
	requestId, err := strconv.Atoi(mux.Vars(r)["request_id"])
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	query := `select id, method, scheme, host, path, header, body from request where id = $1`

	savedReq := &models.Request{}
	byteHeaders := make([]byte, 0)

	err = api.db.QueryRow(query, requestId).Scan(&savedReq.Id, &savedReq.Method, &savedReq.Scheme, &savedReq.Host, &savedReq.Path, &byteHeaders, &savedReq.Body)
	if err != nil {
		return
	}
	var jsonHeaders http.Header
	err = json.Unmarshal(byteHeaders, &jsonHeaders)

	bytesReq, err := json.Marshal(savedReq)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Write(bytesReq)
}

func (api *Api) GetAllRequests(w http.ResponseWriter, r *http.Request) {
	query := `select id, method, scheme, host, path, header, body from request`

	row, err := api.db.Query(query)
	if err != nil {
		log.Println(err)
		return
	}
	defer row.Close()
	savedReqs := make([]*models.Request, 0)

	for row.Next() {
		request := &models.Request{}
		b := make([]byte, 0)

		err = row.Scan(&request.Id, &request.Method, &request.Scheme, &request.Host, &request.Path, &b, &request.Body)
		if err != nil {
			log.Println(err)
			return
		}

		err = json.Unmarshal(b, &request.Headers)
		if err != nil {
			log.Println(err)
			return
		}

		savedReqs = append(savedReqs, request)
	}

	for _, request := range savedReqs {
		bytes, err := json.Marshal(request)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Write(bytes)
		w.Write([]byte("\n\n"))
	}
}

func (api *Api) ScanRequest(w http.ResponseWriter, r *http.Request) {
	requestId, err := strconv.Atoi(mux.Vars(r)["request_id"])
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	query := `select id, method, scheme, host, path, header, body from request where id = $1`

	savedReq := &models.Request{}
	byteHeaders := make([]byte, 0)

	err = api.db.QueryRow(query, requestId).Scan(&savedReq.Id, &savedReq.Method, &savedReq.Scheme, &savedReq.Host, &savedReq.Path, &byteHeaders, &savedReq.Body)
	if err != nil {
		return
	}
	var jsonHeaders http.Header
	err = json.Unmarshal(byteHeaders, &jsonHeaders)

	req := &http.Request{
		Method: savedReq.Method,
		URL: &url.URL{
			Scheme: savedReq.Scheme,
			Host:   savedReq.Host,
			Path:   savedReq.Path,
		},
		Body:   ioutil.NopCloser(strings.NewReader(savedReq.Body)),
		Host:   savedReq.Host,
		Header: jsonHeaders,
	}

	file, err := os.Open("repeater/dicc.txt")
	if err != nil {
		log.Println(err)
	}
	defer func() {
		if err = file.Close(); err != nil {
			log.Println(err)
		}
	}()

	scanner := bufio.NewScanner(file)
	responses := make([]*models.Response, 0)
	i := 0
	for scanner.Scan() {
		i++
		req.URL.Path = "/" + scanner.Text()

		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Println(err)
		}
		req.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))
		resp := api.proxy.HandleHTTP(w, req, true)
		req.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))

		respHeaders, err := json.Marshal(resp.Header)
		if err != nil {
			log.Println(err)
		}
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
		}
		resp.Body = ioutil.NopCloser(bytes.NewBuffer(respBody))

		response := &models.Response{
			Status:  resp.Status,
			Headers: string(respHeaders),
			Body:    string(respBody),
		}

		responses = append(responses, response)
		if i > 5 {
			break
		}
	}

	for _, resp := range responses {
		bytes, err := json.Marshal(resp)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Write(bytes)
		w.Write([]byte("\n\n"))
	}
}
