package repeater

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
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

	if savedReq.Scheme == "http" {
		api.proxy.HandleHTTP(w, req)
	} else if savedReq.Scheme == "https" {
		api.proxy.HandleHTTPS(w, req)
	}
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

	bytesReq, err := json.Marshal(req)
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
