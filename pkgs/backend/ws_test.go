package backend

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"go.acuvity.ai/minibridge/pkgs/backend/client"
	"go.acuvity.ai/minibridge/pkgs/frontend"
	"go.acuvity.ai/minibridge/pkgs/policer"
	"go.acuvity.ai/wsc"
)

func freePort() int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func startBackend(ctx context.Context, opts ...Option) (wsc.Websocket, error) {

	backendListen := fmt.Sprintf("127.0.0.1:%d", freePort())

	srv, err := client.NewMCPServer("cat")
	if err != nil {
		return nil, err
	}

	backend := NewWebSocket(backendListen, nil, srv, opts...)

	go func() {
		if err := backend.Start(ctx); err != nil {
			panic(err)
		}
	}()

	<-time.After(time.Second) // wait a bit.. gh workers are slow

	ws, err := frontend.Connect(ctx, nil, fmt.Sprintf("ws://%s/ws", backendListen), nil, frontend.AgentInfo{UserAgent: "go-test"})
	if err != nil {
		return nil, err
	}

	return ws, nil
}

func TestWS(t *testing.T) {

	Convey("Given a ws backend without policer or tls", t, func() {

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		ws, err := startBackend(ctx)
		So(err, ShouldBeNil)

		echo := `{"hello": "world"}`
		ws.Write([]byte(echo))

		var data []byte
		select {
		case data = <-ws.Read():
		case <-time.After(time.Second):
		}

		So(string(data), ShouldEqual, echo)

		echo = `not-json`
		ws.Write([]byte(echo))

		select {
		case data = <-ws.Read():
		case <-time.After(time.Second):
		}

		So(string(data), ShouldEqual, `{"error":{"code":500,"message":"unable to decode application/json: json decode error [pos 4]: expecting ot-: got ull"},"jsonrpc":"2.0"}`)
	})

	Convey("Given a ws backend with a rego policer that denies the call", t, func() {

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		policer, err := policer.NewRego(`package main
		import rego.v1
		default allow := false
		reasons contains "you can't do that, Dave"
		`)
		So(err, ShouldBeNil)

		ws, err := startBackend(ctx, OptPolicer(policer))
		So(err, ShouldBeNil)

		echo := `{"jsonrpc": "2.0", "id": 2}`
		ws.Write([]byte(echo))

		var data []byte
		select {
		case data = <-ws.Read():
		case <-time.After(time.Second):
		}

		So(string(data), ShouldEqual, `{"error":{"code":451,"message":"request blocked: you can't do that, Dave"},"id":2,"jsonrpc":"2.0"}`)
	})

	Convey("Given a ws backend with a rego policer that allows the call without mutation", t, func() {

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		policer, err := policer.NewRego(`package main
		import rego.v1
		default allow := true
		`)
		So(err, ShouldBeNil)

		ws, err := startBackend(ctx, OptPolicer(policer))
		So(err, ShouldBeNil)

		echo := `{"jsonrpc": "2.0", "id": 1}`
		ws.Write([]byte(echo))

		var data []byte
		select {
		case data = <-ws.Read():
		case <-time.After(time.Second):
		}

		So(string(data), ShouldEqual, `{"jsonrpc": "2.0", "id": 1}`)
	})

	Convey("Given a ws backend with a rego policer that allows the call with mutation", t, func() {

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		policer, err := policer.NewRego(`package main
		import rego.v1
		default allow := true

		mcp := x if {
			x := json.patch(input.mcp, [{
				"op": "replace",
				"path": "/result/hello",
				"value": "world"
			}, {
				"op": "replace",
				"path": "/id",
				"value": 2,
			}])
		}
		`)
		So(err, ShouldBeNil)

		ws, err := startBackend(ctx, OptPolicer(policer))
		So(err, ShouldBeNil)

		echo := `{"id": 1, "jsonrpc": "2.0", "result": {"hello": "monde"}}`
		ws.Write([]byte(echo))

		var data []byte
		select {
		case data = <-ws.Read():
		case <-time.After(time.Second):
		}

		So(string(data), ShouldEqual, `{"id":1,"jsonrpc":"2.0","result":{"hello":"world"}}`)
	})
}
