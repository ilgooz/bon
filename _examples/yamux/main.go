// Demostrates using hashicorp/yamux as a Transporter.
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/ilgooz/bon"
)

type service struct {
	addr    string
	session *yamux.Session
}

func newService(addr string) *service {
	return &service{
		addr: addr,
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

func (s *service) runYamuxServer() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	s.session, err = yamux.Server(conn, nil)
	return err
}

func (s *service) runYamuxClient() error {
	conn, err := net.Dial("tcp", s.addr)
	if err != nil {
		return err
	}
	s.session, err = yamux.Client(conn, nil)
	if err != nil {
		return err
	}
	return nil
}

const (
	// Describe your routes.
	GRPCConn bon.Route = 1 << iota
	ChatConn
)

func main() {
	addr := ":3200"
	service1 := newService(addr)
	service2 := newService(addr)

	// start yamux server and client.
	go func() {
		err := service1.runYamuxServer()
		if err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		err := service2.runYamuxClient()
		if err != nil {
			log.Fatal(err)
		}
	}()

	// wait for connections to get ready.
	time.Sleep(time.Millisecond * 100)

	// register handlers for bon1 and start accepting connections.
	bon1 := bon.New(service1)
	bon1.Handle(GRPCConn, func(conn net.Conn) {
		_, err := conn.Write([]byte("grpc"))
		if err != nil {
			log.Fatal(err)
		}
		conn.Close()
	})
	go bon1.Run()

	// register handlers for bon2 and start accepting connections.
	bon2 := bon.New(service2)
	bon2.Handle(ChatConn, func(conn net.Conn) {
		_, err := conn.Write([]byte("chat"))
		if err != nil {
			log.Fatal(err)
		}
		conn.Close()
	})
	go bon2.Run()

	// read handler response from bon2
	conn, err := bon1.Connect(ChatConn)
	if err != nil {
		log.Fatal(err)
	}
	data, err := ioutil.ReadAll(conn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))

	// read handler response from bon1
	conn, err = bon2.Connect(GRPCConn)
	if err != nil {
		log.Fatal(err)
	}
	data, err = ioutil.ReadAll(conn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))

	// cleanup
	if err := bon1.Close(); err != nil {
		log.Fatal(err)
	}
	if err := bon2.Close(); err != nil {
		log.Fatal(err)
	}
}
