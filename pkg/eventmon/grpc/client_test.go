// This package is for testing the grpc client calls to the hello service
package grpc_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/dusk-network/dusk-protobuf/autogen/go/monitor"
	"github.com/stretchr/testify/assert"
	g "google.golang.org/grpc"
)

type helloSrv struct {
	requestChan chan interface{}
	srv         *g.Server
}

type callTest struct{
	clientMethod  func() error
	tester func(interface{}) error
}

var emptyFunc = func(_ interface{}) error {
	return nil
}

func newSrv(network, addr string) *helloSrv {
	l, err := net.Listen(network, addr)
	if err != nil {
		panic(err)
	}

	semverChan := make(chan interface{}, 1)
	grpcServer := g.NewServer()
	hs := &helloSrv{semverChan, grpcServer}
	monitor.RegisterMonitorServer(grpcServer, hs)
	// This function is blocking, so we run it in a goroutine
	go grpcServer.Serve(l)
	return hs
}

// Suite automates testing of BlockUpdates received through the grpc call. It
// accepts a clientMethod to prep the test, and a varargs of tester functions
// which apply to the payload received. Each tester is supposed to test a
// correspondent payload
func Suite(t *testing.T, timeoutMillis time.Duration, calls ...callTest) {
	semverSrv := newSrv("tcp", ":7878")
	defer semverSrv.srv.Stop()
	time.Sleep(10 * time.Millisecond)

	for i, call := range calls {
		if !assert.NoError(t, call.clientMethod()) {
			t.FailNow()
		}
		select {
		case response := <-semverSrv.requestChan:
			if !assert.NoError(t, call.tester(response)) {
				t.FailNow()
			}
		case <-time.After(timeoutMillis * time.Millisecond):
			assert.FailNow(t, fmt.Sprintf("timeout in receiving packet #%d", i))
		}
	}
}
