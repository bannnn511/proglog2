package server

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

func buildAPIPath(path string) string {
	return "/api/v1" + path
}

func NewHTTPServer(addr string) http.Server {
	r := chi.NewRouter()
	httpServer := newHttpServer()
	r.Get(buildAPIPath("/health"), func(writer http.ResponseWriter, request *http.Request) {
		writer.Header()
		_, err := writer.Write([]byte("OK"))
		if err != nil {
			panic(err)
		}
	})

	r.Post(buildAPIPath("/produce"), httpServer.produceHandler)
	r.Get(buildAPIPath("/consume"), httpServer.consumeHandler)

	return http.Server{
		Addr:    addr,
		Handler: r,
	}
}

type httpServer struct {
	Log *Log
}

func newHttpServer() *httpServer {
	return &httpServer{
		Log: NewLog(),
	}
}

type ProduceRequest struct {
	Record Record `json:"record"`
}

type ProduceResponse struct {
	Offset uint64 `json:"offset"`
}

func (s *httpServer) produceHandler(write http.ResponseWriter, request *http.Request) {
	var body ProduceRequest
	err := json.NewDecoder(request.Body).Decode(&body)
	if err != nil {
		http.Error(write, err.Error(), http.StatusInternalServerError)
		return
	}

	offset, err := s.Log.Append(body.Record)
	if err != nil {
		http.Error(write, err.Error(), http.StatusInternalServerError)
		return
	}

	response := ProduceResponse{
		offset,
	}
	err = json.NewEncoder(write).Encode(response)
	if err != nil {
		http.Error(write, err.Error(), http.StatusInternalServerError)
		return
	}
}

type consumeResponse struct {
	Record Record `json:"record"`
}

func (s *httpServer) consumeHandler(write http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	offsetStr, ok := query["offset"]
	if !ok || len(offsetStr) == 0 {
		http.Error(write,
			"offset is required in params",
			http.StatusBadRequest)
		return
	}
	offset, err := strconv.ParseUint(offsetStr[0], 10, 64)
	if err != nil {
		http.Error(write, err.Error(), http.StatusBadRequest)
		return
	}
	record, err := s.Log.Read(offset)
	if err != nil {
		http.Error(write, err.Error(), http.StatusInternalServerError)
		return
	}

	consumeResponse := consumeResponse{
		Record: record,
	}

	err = json.NewEncoder(write).Encode(consumeResponse)
	if err != nil {
		http.Error(write, err.Error(), http.StatusInternalServerError)
		return
	}
}
