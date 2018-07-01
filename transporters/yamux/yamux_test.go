package yamux

import (
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ilgooz/bon"
	"github.com/stretchr/testify/assert"
)

func TestClientServer(t *testing.T) {
	var wg sync.WaitGroup
	r := bon.Route(0)

	server, err := New(ServerOption(":0"))
	if err != nil {
		log.Fatal(err)
	}
	go server.Run()

	wg.Add(1)
	go func() {
		bon := <-server.Bons
		assert.NotNil(t, bon)

		wg.Add(1)
		bon.Handle(r, func(conn net.Conn) {
			assert.NotNil(t, conn)
			wg.Done()
		})
		go bon.Run()

		assert.Nil(t, <-server.Bons)
		wg.Done()
	}()

	// wait for server to start
	time.Sleep(time.Millisecond * 100)

	client, err := New(ClientOption(server.ServerAddr().String()))
	if err != nil {
		log.Fatal(err)
	}
	go client.Run()

	bon := <-client.Bons
	assert.NotNil(t, bon)

	conn, err := bon.Connect(r)
	assert.Nil(t, err)
	assert.NotNil(t, conn)

	assert.Nil(t, <-client.Bons)
	server.Close()

	wg.Wait()
}
