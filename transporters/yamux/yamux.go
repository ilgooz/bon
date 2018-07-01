// Package yamux is Bon Transporter based on hashicorp/yamux.
package yamux

import (
	"net"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/ilgooz/bon"
)

// Yamux provides functionalities to use hashicorp/yamux as a connection layer.
type Yamux struct {
	// Bons filled everytime there is matching connection.
	Bons chan *bon.Bon

	options     *opts
	dialTimeout time.Duration

	ln net.Listener
}

type opts struct {
	address     string
	isServer    bool
	dialTimeout time.Duration
	yamux       *yamux.Config
	conn        net.Conn
}

// New creates a new Yamux with given options.
func New(options ...Option) (*Yamux, error) {
	y := &Yamux{
		options: &opts{
			dialTimeout: time.Second * 5,
		},
		Bons: make(chan *bon.Bon, 100),
	}
	for _, o := range options {
		o(y)
	}
	return y, nil
}

// Option is a Yamux option.
type Option func(*Yamux)

// ServerOption makes Yamux a TCP server.
func ServerOption(address string) Option {
	return func(y *Yamux) {
		y.options.address = address
		y.options.isServer = true
	}
}

// ClientOption func makes Yamux a TCP client.
func ClientOption(address string) Option {
	return func(y *Yamux) {
		y.options.address = address
	}
}

// ConnOption receives a conn that will be used as the underlying connection for Yamux.
// You should't use ServerOption or ClientOption with this.
func ConnOption(conn net.Conn) Option {
	return func(y *Yamux) {
		y.options.conn = conn
	}
}

// DialTimeoutOption is used to timeout while connectig to server.
func DialTimeoutOption(d time.Duration) Option {
	return func(y *Yamux) {
		y.options.dialTimeout = d
	}
}

// YamuxConfigOption passed directly to hashicorp/yamux.
func YamuxConfigOption(c *yamux.Config) Option {
	return func(y *Yamux) {
		y.options.yamux = c
	}
}

// Run starts server or client.
func (y *Yamux) Run() error {
	if y.options.isServer {
		if y.options.conn != nil {
			return y.setupYamuxServer(y.options.conn)
		}
		return y.handleServer()
	}

	if y.options.conn != nil {
		return y.setupYamuxClient(y.options.conn)
	}
	return y.handleClient()
}

func (y *Yamux) handleServer() error {
	var err error
	y.ln, err = net.Listen("tcp", y.options.address)
	if err != nil {
		return err
	}
	for {
		conn, err := y.ln.Accept()
		if err != nil {
			return err
		}
		err = y.setupYamuxServer(conn)
		if err != nil {
			return err
		}
	}
}

func (y *Yamux) handleClient() error {
	conn, err := net.DialTimeout("tcp", y.options.address, y.dialTimeout)
	if err != nil {
		return err
	}
	return y.setupYamuxClient(conn)
}

func (y *Yamux) setupYamuxServer(conn net.Conn) error {
	s, err := yamux.Server(conn, y.options.yamux)
	if err != nil {
		return err
	}
	go y.handleSession(s)
	return nil
}

func (y *Yamux) setupYamuxClient(conn net.Conn) error {
	s, err := yamux.Client(conn, nil)
	if err != nil {
		return err
	}
	go y.handleSession(s)
	return nil
}

func (y *Yamux) handleSession(s *yamux.Session) {
	srv := newService(s)
	b := bon.New(srv)
	y.Bons <- b
	if !y.options.isServer {
		close(y.Bons)
	}
}

// Close stops accepting new connections for server.
func (y *Yamux) Close() error {
	if y.options.isServer {
		close(y.Bons)
		return y.ln.Close()
	}
	return nil
}

// ServerAddr returns server's address if Yamux started as a server.
func (y *Yamux) ServerAddr() net.Addr {
	if y.options.isServer {
		return y.ln.Addr()
	}
	return nil
}
