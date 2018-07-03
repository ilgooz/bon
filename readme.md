# bon [![GoDoc](https://godoc.org/github.com/ilgooz/bon?status.svg)](https://godoc.org/github.com/ilgooz/bon) [![Go Report Card](https://goreportcard.com/badge/github.com/ilgooz/bon)](https://goreportcard.com/report/github.com/ilgooz/bon) [![Build Status](https://travis-ci.org/ilgooz/bon.svg?branch=master)](https://travis-ci.org/ilgooz/bon)

Bon provides routing capability for your net.Conn's like you do with for your
http handlers. It can both accept and open connections like described in Transporter.
Thus, you can both Connect to a route and invoke a handler of one when requested by others.

See [Transporter](https://godoc.org/github.com/ilgooz/bon/#Transporter) to implement your own net.Conn provider.

```
go get gopkg.in/ilgooz/bon.v1
```
## Usage

```go
const (
  // Define your routes.
  GRPCConn bon.Route = 1 << iota
)

remoteService := bon.New(remoteServiceTransporter)
remoteService.Handle(GRPCConn, func(conn net.Conn){
  // do domething with your conn...
})
go remoteService.Run()


service := bon.New(serviceTransporter)
conn, err := service.Connect(GRPCConn)
// do domething with your conn...
```
