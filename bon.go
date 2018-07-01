// Package bon provides routing capability for your net.Conn's like you do with for your
// http handlers. It can both accept and open connections like described in Transporter.
// Thus, you can both Connect to a route and invoke a handler of one when requested by others.
package bon

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
)

// Route provides type safety when you describe your connection routes for handlers.
type Route uint32

// Transporter describes how your connection provider should be. Since it has both
// Open and Accept methods your provider must behave like a Client and a Server
// at the same time.
//
// Transporter should be gorutinue safe.
type Transporter interface {
	// Open starts a new connection for Route. You can make logical decisions about
	// which server or multiplex connection you will use to serve your new connection
	// depending on value of Route.
	//
	// If you don't have multiple server environment that each server handles different
	// kind of routes you can safely ignore exploring the Route argument.
	Open(Route) (net.Conn, error)

	// Accept will accept connections whenever any available.
	Accept() (net.Conn, error)

	// Close should cancel new connections for Accept.
	Close() error
}

// Bon provides functionalities to register your handlers and start new connections for your Routes.
type Bon struct {
	// transporter holds Transporter
	transporter Transporter

	handlers map[Route]func(net.Conn)
	hm       sync.RWMutex

	nonMatchingHandler func(net.Conn)
	nhm                sync.RWMutex

	// options keeps user options for Bon
	options *opts

	// log
	log *log.Logger
}

// New expects a Transporter as a net.Conn provider. Since it just cares about net.Conn's,
// you can do connection pooling or multiplexing on your side.
//
// Provided options will be applied to Bon as configuration.
func New(t Transporter, options ...Option) *Bon {
	b := &Bon{
		transporter: t,
		handlers:    make(map[Route]func(net.Conn)),
		options: &opts{
			logOutput: os.Stdout,
		},
	}
	for _, optionFunc := range options {
		optionFunc(b)
	}
	b.log = log.New(b.options.logOutput, "bon", log.LstdFlags)
	return b
}

// Option is the configuration function for Bon.
type Option func(*Bon)

type opts struct {
	logOutput io.Writer
}

// LogOutputOption uses out as a log destination.
func LogOutputOption(out io.Writer) Option {
	return func(b *Bon) {
		b.options.logOutput = out
	}
}

// Handle will handle connections for provided r. If there is no matching handlers and
// HandleNonMatching not set, the sender will have a HandlerError.
func (b *Bon) Handle(r Route, h func(net.Conn)) {
	b.hm.Lock()
	defer b.hm.Unlock()
	b.handlers[r] = h
}

// HandleNonMatching will handle connections that doesn't match any route. If you don't register
// a handler here, the sender will have a HandlerError.
func (b *Bon) HandleNonMatching(h func(net.Conn)) {
	b.nhm.Lock()
	defer b.nhm.Unlock()
	b.nonMatchingHandler = h
}

// Off will remove the registered handler for r.
func (b *Bon) Off(r Route) {
	b.hm.Lock()
	defer b.hm.Unlock()
	delete(b.handlers, r)
}

// OffNonMatching will remove non-matching handler.
func (b *Bon) OffNonMatching() {
	b.nhm.Lock()
	defer b.nhm.Unlock()
	b.nonMatchingHandler = nil
}

const (
	handlerExists uint32 = 1 << iota
	nonMatchingHandlerExists
	handlerDoesNotExists
)

// Connect opens a new connection for given route. If there is no handler for r at the
// receiver's end an error will be returned.
func (b *Bon) Connect(r Route) (net.Conn, error) {
	conn, err := b.transporter.Open(r)
	if err != nil {
		return nil, err
	}

	// tell receiver which handler we want to use
	err = b.writeUInt32(conn, uint32(r))
	if err != nil {
		return nil, err
	}

	// get an answer from receiver if we got a handler match
	// if we do we can return the conn to user.
	data, err := b.readUInt32(conn)
	if err != nil {
		return nil, err
	}

	switch data {
	case handlerExists:
		return conn, nil
	case nonMatchingHandlerExists:
		return conn, nil
	}
	return nil, HandlerError{r}
}

// Run accepts incoming connections. It blocks till a network failure then
// returns an error.
func (b *Bon) Run() error {
	for {
		conn, err := b.transporter.Accept()
		if err != nil {
			return err
		}
		go b.handleConn(conn)
	}
}

// Close cancels accepting new connections.
func (b *Bon) Close() error {
	return b.transporter.Close()
}

// handleConn calls corresponding handler when requested Route is matching or
// the non-matching handler is registered.
//
// It will silently die if the connection gets broken before invoking the handler.
func (b *Bon) handleConn(conn net.Conn) {
	// get Route id of requested handler
	data, err := b.readUInt32(conn)
	if err != nil {
		b.log.Println(err)
		return
	}

	handlerExistence := handlerDoesNotExists
	b.hm.RLock()
	h := b.handlers[Route(data)]
	b.hm.RUnlock()

	if h != nil {
		handlerExistence = handlerExists
	} else {
		b.nhm.RLock()
		h = b.nonMatchingHandler
		b.nhm.RUnlock()

		if h != nil {
			handlerExistence = nonMatchingHandlerExists
			h = b.nonMatchingHandler
		}
	}

	// tell to the sender if we got a matching handler or not.
	err = b.writeUInt32(conn, handlerExistence)
	if err != nil {
		b.log.Println(err)
		return
	}

	if h != nil {
		h(conn)
	} else {
		err := conn.Close()
		if err != nil {
			b.log.Println(err)
		}
	}
}

func (b *Bon) writeUInt32(conn net.Conn, data uint32) error {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, data)
	_, err := conn.Write(buf)
	return err
}

func (b *Bon) readUInt32(conn net.Conn) (data uint32, err error) {
	buf := make([]byte, 4)
	_, err = conn.Read(buf)
	if err != nil {
		return 0, err
	}
	s := binary.BigEndian.Uint32(buf)
	return uint32(s), nil
}

// HandlerError implements error interface and produced as a return value of
// Connect if there is no matching handler and HandleNonMatching is not set for
// given Route at the receiver's end.
type HandlerError struct {
	// Route is the non-matching Route.
	Route Route
}

// Error will return an error in string format.
func (e HandlerError) Error() string {
	return fmt.Sprintf("no `%d` handler does not exists.", e.Route)
}
