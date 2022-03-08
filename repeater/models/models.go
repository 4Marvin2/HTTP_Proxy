package models

type Request struct {
	Id      int64
	Method  string
	Scheme  string
	Host    string
	Path    string
	Headers map[string][]string
	Body    string
}

type Response struct {
	Id      int64
	Status  string
	Headers string
	Body    string
}
