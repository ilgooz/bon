package bon

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	try "gopkg.in/matryer/try.v1"
)

func init() {
	try.MaxRetries = 30
}

type testConn struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func (c *testConn) Close() error {
	err := c.r.Close()
	if err != nil {
		return err
	}
	return c.w.Close()
}

func (c *testConn) Read(data []byte) (n int, err error) {
	return c.r.Read(data)
}

func (c *testConn) Write(data []byte) (n int, err error) {
	return c.w.Write(data)
}

func (c *testConn) LocalAddr() net.Addr {
	return &net.IPAddr{}
}

func (c *testConn) RemoteAddr() net.Addr {
	return &net.IPAddr{}
}

func (c *testConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *testConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *testConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func newConnectedTestConns() (net.Conn, net.Conn) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()

	return &testConn{
			r1,
			w2,
		}, &testConn{
			r2,
			w1,
		}
}

type provider struct {
	connSendC chan net.Conn
	connRecvC chan net.Conn
	closeC    chan chan struct{}
}

func newProvider(connSendC, connRecvC chan net.Conn) *provider {
	return &provider{
		connSendC, connRecvC,
		make(chan chan struct{}, 0),
	}
}

func (p *provider) Open(r Route) (net.Conn, error) {
	c1, c2 := newConnectedTestConns()
	select {
	case p.connSendC <- c1:
	default:
		return nil, ErrNet
	}
	return c2, nil
}

func (p *provider) Accept() (net.Conn, error) {
	select {
	case conn := <-p.connRecvC:
		return conn, nil
	case closeReqC := <-p.closeC:
		defer func() { closeReqC <- struct{}{} }()
		return nil, ErrNet
	}
}

func (p *provider) Close() error {
	closeReqC := make(chan struct{}, 0)
	select {
	case p.closeC <- closeReqC:
		<-closeReqC
	default:
	}

	close(p.closeC)
	return nil
}

func newBons(t *testing.T) (*Bon, *Bon) {
	connC1 := make(chan net.Conn, 0)
	connC2 := make(chan net.Conn, 0)

	p1 := newProvider(connC1, connC2)
	p2 := newProvider(connC2, connC1)

	return New(p1), New(p2)
}

var ErrNet = errors.New("net error")

func connect(b *Bon, r Route) (net.Conn, error) {
	var conn net.Conn
	var err error

	err = try.Do(func(attempt int) (bool, error) {
		time.Sleep(time.Millisecond * 2)
		conn, err = b.Connect(r)
		return attempt < 30, err
	})

	return conn, err
}
func TestHandleNonMatching(t *testing.T) {
	r := Route(0)
	b1, b2 := newBons(t)

	var wg sync.WaitGroup
	wg.Add(1)
	b1.HandleNonMatching(func(conn net.Conn) {
		assert.NotNil(t, conn)
		wg.Done()
	})
	go b1.Run()

	conn, err := connect(b2, r)
	assert.Nil(t, err)
	assert.NotNil(t, conn)

	b1.OffNonMatching()
	conn, err = connect(b2, r)
	assert.Equal(t, HandlerError{r}, err)
	assert.Nil(t, conn)

	wg.Wait()
}

func TestHandle(t *testing.T) {
	r := Route(1)
	b1, b2 := newBons(t)
	var wg sync.WaitGroup

	wg.Add(1)
	b1.Handle(r, func(conn net.Conn) {
		assert.NotNil(t, conn)
		wg.Done()
	})
	go b1.Run()

	var conn net.Conn
	var err error

	try.Do(func(attempt int) (bool, error) {
		time.Sleep(time.Millisecond)
		var err error
		conn, err = b2.Connect(r)
		return attempt < 100, err
	})

	assert.Nil(t, err)
	assert.NotNil(t, conn)

	wg.Wait()
}

func TestNonExistentHandle(t *testing.T) {
	r := Route(0)
	b1, b2 := newBons(t)
	go b1.Run()

	conn, err := connect(b2, r)
	assert.Equal(t, HandlerError{r}, err)
	assert.Nil(t, conn)
}

func TestOff(t *testing.T) {
	r := Route(0)
	b1, b2 := newBons(t)

	b1.Handle(r, func(conn net.Conn) {
		assert.Fail(t, "should be removed")
	})
	go b1.Run()

	b1.Off(r)

	conn, err := connect(b2, r)
	assert.Equal(t, HandlerError{r}, err)
	assert.Nil(t, conn)
}

func TestReadWrite(t *testing.T) {
	r := Route(0)
	d1 := []byte{'b', 'o', 'n', 'n'}
	d2 := make([]byte, 4)

	b1, b2 := newBons(t)

	b1.Handle(r, func(conn net.Conn) {
		conn.Write(d1)
	})
	go b1.Run()

	conn, err := connect(b2, r)
	assert.Nil(t, err)
	_, err = conn.Read(d2)
	assert.Nil(t, err)
	assert.Equal(t, d1, d2)
}

func TestMultipleHandle(t *testing.T) {
	r1 := Route(0)
	r2 := Route(1)
	r3 := Route(2)
	d1 := []byte{'t', 'u', 'n', 'n', 'l', '.', 'r', 'o', 'c', 'k', 's'}
	d2 := []byte{'h', 'e', 'l', 'l', 'o'}
	d3 := []byte{'g', 'o'}
	d4 := []byte{'d', 'o', 'c'}
	dx := make([]byte, 11)
	dy := make([]byte, 5)
	dz := make([]byte, 2)
	dt := make([]byte, 3)

	b1, b2 := newBons(t)

	b1.Handle(r1, func(conn net.Conn) {
		conn.Write(d1)
	})
	b1.Handle(r2, func(conn net.Conn) {
		conn.Write(d2)
	})
	b2.Handle(r2, func(conn net.Conn) {
		conn.Write(d3)
	})
	b2.Handle(r3, func(conn net.Conn) {
		conn.Write(d4)
	})
	go b1.Run()
	go b2.Run()

	conn, err := connect(b2, r1)
	assert.Nil(t, err)
	_, err = conn.Read(dx)
	assert.Nil(t, err)
	assert.Equal(t, d1, dx)
	assert.Nil(t, conn.Close())

	conn, err = connect(b2, r2)
	assert.Nil(t, err)
	_, err = conn.Read(dy)
	assert.Nil(t, err)
	assert.Equal(t, d2, dy)
	assert.Nil(t, conn.Close())

	conn, err = connect(b2, r3)
	assert.NotNil(t, HandlerError{r3})
	assert.Nil(t, conn)

	conn, err = connect(b1, r2)
	assert.Nil(t, err)
	_, err = conn.Read(dz)
	assert.Nil(t, err)
	assert.Equal(t, d3, dz)
	assert.Nil(t, conn.Close())

	conn, err = connect(b1, r3)
	assert.Nil(t, err)
	_, err = conn.Read(dt)
	assert.Nil(t, err)
	assert.Equal(t, d4, dt)
	assert.Nil(t, conn.Close())
}

func TestCloseAfterRun(t *testing.T) {
	r := Route(0)
	b1, b2 := newBons(t)
	b1.Handle(r, func(net.Conn) {})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		assert.Equal(t, ErrNet, b1.Run())
		wg.Done()
	}()

	conn, err := connect(b2, r)
	assert.Nil(t, err)
	assert.NotNil(t, conn)
	assert.Nil(t, b1.Close())

	wg.Wait()
}

func TestSelfCloseRunRemoteConnect(t *testing.T) {
	r := Route(0)

	b1, b2 := newBons(t)
	go b1.Run()
	b1.Close()

	conn, err := connect(b2, r)
	assert.Equal(t, ErrNet, err)
	assert.Nil(t, conn)
}

func TestSelfCloseRemoteConnect(t *testing.T) {
	r := Route(0)

	b1, b2 := newBons(t)
	b1.Close()

	conn, err := connect(b2, r)
	assert.Equal(t, ErrNet, err)
	assert.Nil(t, conn)
}

func TestSelfCloseSelfConnect(t *testing.T) {
	r := Route(0)

	b1, b2 := newBons(t)
	go b1.Run()
	go b2.Run()
	b1.Close()

	conn, err := connect(b1, r)
	assert.Equal(t, HandlerError{r}, err)
	assert.Nil(t, conn)
}

func TestConnectWithoutRemoteRun(t *testing.T) {
	r := Route(0)

	b1, _ := newBons(t)
	b1.Close()

	conn, err := connect(b1, r)
	assert.Equal(t, ErrNet, err)
	assert.Nil(t, conn)
}
