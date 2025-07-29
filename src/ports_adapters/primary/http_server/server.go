package http_server

import "net/http"

type Server struct {
	// TODO: добавьте зависимости сервера
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, nil)
}