package yamux

import (
	"net"

	"github.com/hashicorp/yamux"
	"github.com/ilgooz/bon"
)

type service struct {
	session *yamux.Session
}

func newService(session *yamux.Session) *service {
	return &service{
		session: session,
	}
}

func (s *service) Accept() (net.Conn, error) {
	return s.session.Accept()
}

func (s *service) Open(r bon.Route) (net.Conn, error) {
	return s.session.Open()
}

func (s *service) Close() error {
	return s.session.Close()
}
