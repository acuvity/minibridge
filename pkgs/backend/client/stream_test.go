package client

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"go.acuvity.ai/minibridge/pkgs/mcp"
)

func TestMCPStream(t *testing.T) {

	Convey("I have a running stream, registrations should work", t, func() {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()

		stream := NewMCPStream(ctx)

		stdout1, closeStdout1 := stream.Stdout()
		defer closeStdout1()
		stdout2, closeStdout2 := stream.Stdout()
		defer closeStdout2()

		stderr1, closeStderr1 := stream.Stderr()
		defer closeStderr1()
		stderr2, closeStderr2 := stream.Stderr()
		defer closeStderr2()

		exit1, closeExit1 := stream.Exit()
		defer closeExit1()
		exit2, closeExit2 := stream.Exit()
		defer closeExit2()

		cstdout1 := make(chan []byte)
		go func() { cstdout1 <- <-stdout1 }()
		cstdout2 := make(chan []byte)
		go func() { cstdout2 <- <-stdout2 }()
		cstderr1 := make(chan []byte)
		go func() { cstderr1 <- <-stderr1 }()
		cstderr2 := make(chan []byte)
		go func() { cstderr2 <- <-stderr2 }()
		cexit1 := make(chan error)
		go func() { cexit1 <- <-exit1 }()
		cexit2 := make(chan error)
		go func() { cexit2 <- <-exit2 }()

		go func() {
			stream.stdout <- []byte("hello stdout")
			stream.stderr <- []byte("hello stderr")
			stream.exit <- fmt.Errorf("hello from error")
		}()

		So(string(<-cstdout1), ShouldEqual, "hello stdout")
		So(string(<-cstdout2), ShouldEqual, "hello stdout")
		So(string(<-cstderr1), ShouldEqual, "hello stderr")
		So(string(<-cstderr2), ShouldEqual, "hello stderr")
		So((<-cexit1).Error(), ShouldEqual, "hello from error")
		So((<-cexit2).Error(), ShouldEqual, "hello from error")

		// let's unregister all 1s
		// this will test that all channel will
		// correctly unregister and will not block
		// the rest.
		closeStdout1()
		closeStderr1()
		closeExit1()

		go func() { cstdout2 <- <-stdout2 }()
		go func() { cstderr2 <- <-stderr2 }()
		go func() { cexit2 <- <-exit2 }()

		go func() {
			stream.stdout <- []byte("hello stdout 2")
			stream.stderr <- []byte("hello stderr 2")
			stream.exit <- fmt.Errorf("hello from error 2")
		}()

		So(string(<-cstdout2), ShouldEqual, "hello stdout 2")
		So(string(<-cstderr2), ShouldEqual, "hello stderr 2")
		So((<-cexit2).Error(), ShouldEqual, "hello from error 2")
	})

	Convey("SendRequest should work", t, func() {

		stream := NewMCPStream(t.Context())

		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()

		done := make(chan bool, 1)
		cstdin := make(chan []byte, 1)
		go func() {
			cstdin <- <-stream.stdin
			stream.stdout <- []byte(`{"id":"not-id"}`)
			stream.stdout <- []byte(`{"id":"not-id-again-3"}`)
			stream.stdout <- []byte(`{"id":"id","result":{"a":1}}`)
			stream.stdout <- []byte(`{"id":"not-id-again-4"}`)
			done <- true
		}()

		resp, err := stream.SendRequest(ctx, mcp.NewMessage("id"))

		So(string(<-cstdin), ShouldResemble, `{"id":"id","jsonrpc":"2.0"}`)

		So(err, ShouldBeNil)
		So(resp, ShouldNotBeNil)
		So(resp.ID, ShouldEqual, "id")
		So(resp.Result["a"], ShouldEqual, 1)
		So(<-done, ShouldBeTrue)
	})

	Convey("SendRequest should work when context cancels before sending data", t, func() {

		stream := NewMCPStream(t.Context())

		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		cancel()

		_, err := stream.SendRequest(ctx, mcp.NewMessage("id"))
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "unable to send request: context canceled")
	})

	Convey("SendRequest should work when context cancels while awaiting for response", t, func() {

		stream := NewMCPStream(t.Context())

		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()

		done := make(chan bool, 1)
		cstdin := make(chan []byte, 1)
		go func() {
			cstdin <- <-stream.stdin
			stream.stdout <- []byte(`{"id":"not-id"}`)
			stream.stdout <- []byte(`{"id":"not-id-again-3"}`)
			stream.stdout <- []byte(`{"id":"not-id-again-4"}`)
			cancel()
			done <- true
		}()

		_, err := stream.SendRequest(ctx, mcp.NewMessage("id"))

		So(string(<-cstdin), ShouldResemble, `{"id":"id","jsonrpc":"2.0"}`)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "unable to get response: context canceled")
	})

	Convey("SendRequest should handle invalid json response for summary", t, func() {

		stream := NewMCPStream(t.Context())

		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()

		go func() {
			<-stream.stdin
			stream.stdout <- []byte(`"id":"not-id"}`)
		}()

		_, err := stream.SendRequest(ctx, mcp.NewMessage("id"))

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldStartWith, "unable to decode mcp call as summary: ")
	})

	Convey("SendRequest should handle invalid json response for mcp call", t, func() {

		stream := NewMCPStream(t.Context())

		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()

		go func() {
			<-stream.stdin
			stream.stdout <- []byte(`{"id":"id", "result": "not a map"}`)
		}()

		_, err := stream.SendRequest(ctx, mcp.NewMessage("id"))

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldStartWith, "unable to decode mcp call: ")
	})

	Convey("calling SendPaginatedRequest should work", t, func() {

		stream := NewMCPStream(t.Context())

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		// enqueue the resp immediately
		cstdin := make(chan []byte, 3)
		go func() {
			cstdin <- <-stream.stdin
			stream.stdout <- []byte(`{"id":"a","jsonrpc":"2.0","result":{"nextCursor":"1"}}`)
			cstdin <- <-stream.stdin
			stream.stdout <- []byte(`{"id":"a","jsonrpc":"2.0"}`)
		}()

		calls, err := stream.SendPaginatedRequest(ctx, mcp.NewMessage("a"))
		So(err, ShouldBeNil)
		So(len(calls), ShouldEqual, 2)

		So(string(<-cstdin), ShouldEqual, `{"id":"a","jsonrpc":"2.0"}`)
		So(string(<-cstdin), ShouldEqual, `{"id":"a","jsonrpc":"2.0","params":{"cursor":"1"}}`)
	})

	Convey("calling SendPaginatedRequest while context cancels should work", t, func() {

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()

		stream := NewMCPStream(ctx)

		sctx, cancel := context.WithCancel(t.Context())
		cancel()

		// enqueue the resp immediately
		go func() { stream.stdout <- []byte(`{"id":46,"jsonrpc":"2.0","result":{"nextCursor":"1"}}`) }()

		calls, err := stream.SendPaginatedRequest(sctx, mcp.NewMessage(44))
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "unable to send paginated request: unable to send request: context canceled")
		So(len(calls), ShouldEqual, 0)
	})
}
