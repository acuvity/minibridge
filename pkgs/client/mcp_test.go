package client

import (
	"context"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
)

func TestMCPServer(t *testing.T) {

	Convey("calling NewMCPServer on existing bin should work", t, func() {
		srv, err := NewMCPServer("echo", "hello")
		So(err, ShouldBeNil)
		So(srv.Command, ShouldEqual, "/usr/bin/echo")
		So(srv.Args, ShouldResemble, []string{"hello"})
	})

	Convey("calling NewMCPServer on non exiting bin should work", t, func() {
		_, err := NewMCPServer("not-echo", "hello")
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, `unable to find server binary: exec: "not-echo": executable file not found in $PATH`)
	})
}

func TestMCPStream(t *testing.T) {

	Convey("I have a MCPStream", t, func() {

		stdin := make(chan []byte, 2)
		stdout := make(chan []byte, 2)
		stderr := make(chan []byte, 1)
		errCh := make(chan error, 1)

		stream := MCPStream{
			Stdin:  stdin,
			Stdout: stdout,
			Stderr: stderr,
			Exit:   errCh,
		}

		Convey("I send a call, it should reach stdin", func() {
			err := stream.Send(api.NewMCPCall(42))
			So(err, ShouldBeNil)
			So(string(<-stdin), ShouldEqual, `{"id":42,"jsonrpc":"2.0"}`)
		})

		Convey("I read a call, it should reach stdout", func() {

			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			stdout <- []byte(`{"id":43,"jsonrpc":"2.0"}`)
			call, err := stream.Read(ctx)
			So(err, ShouldBeNil)
			So(call.ID, ShouldEqual, 43)
		})

		Convey("I read a call,  but it timeout", func() {

			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			cancel()

			_, err := stream.Read(ctx)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldEqual, "context canceled")
		})

		Convey("calling Roundtrip should work", func() {

			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			// enqueue the resp immediately
			stdout <- []byte(`{"id":45,"jsonrpc":"2.0"}`)

			call, err := stream.Roundtrip(ctx, api.NewMCPCall(44))
			So(err, ShouldBeNil)
			So(string(<-stdin), ShouldEqual, `{"id":44,"jsonrpc":"2.0"}`)
			So(call.ID, ShouldEqual, 45)
		})

		Convey("calling PRoundtrip should work", func() {

			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			// enqueue the resp immediately
			stdout <- []byte(`{"id":46,"jsonrpc":"2.0","result":{"nextCursor":"1"}}`)
			stdout <- []byte(`{"id":46,"jsonrpc":"2.0"}`)

			calls, err := stream.PRoundtrip(ctx, api.NewMCPCall(44))
			So(err, ShouldBeNil)
			So(len(calls), ShouldEqual, 2)

			So(string(<-stdin), ShouldEqual, `{"id":44,"jsonrpc":"2.0"}`)
			So(string(<-stdin), ShouldEqual, `{"id":45,"jsonrpc":"2.0","params":{"cursor":"1"}}`)

		})

		Convey("calling PRoundtrip while context cancels should work", func() {

			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			// enqueue the resp immediately
			stdout <- []byte(`{"id":46,"jsonrpc":"2.0","result":{"nextCursor":"1"}}`)
			cancel()

			calls, err := stream.PRoundtrip(ctx, api.NewMCPCall(44))
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldEqual, "context canceled")
			So(len(calls), ShouldEqual, 0)
		})
	})
}
