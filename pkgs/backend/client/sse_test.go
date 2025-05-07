package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSSEClient(t *testing.T) {

	Convey("Type is correct", t, func() {
		cl := NewSSE("https://127.0.0.1", nil)
		So(cl.Type(), ShouldEqual, "sse")
	})

	Convey("Request fails to be made", t, func() {

		cl := NewSSE("not-http://789.11.22.11", nil)

		pipe, err := cl.Start(nil) // nolint
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "unable to initiate request: net/http: nil Context")
		So(pipe, ShouldBeNil)
	})

	Convey("Server does not respond", t, func() {

		cl := NewSSE("http://789.11.22.11", nil)

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()

		pipe, err := cl.Start(ctx)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "unable to send initial sse request: Get \"http://789.11.22.11/sse\": dial tcp: lookup 789.11.22.11: no such host")
		So(pipe, ShouldBeNil)
	})

	Convey("Server does not return a message endpoint in time", t, func() {

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		}))

		cl := NewSSE(ts.URL, nil)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		pipe, err := cl.Start(ctx)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "did not receive /message endpoint in time")
		So(pipe, ShouldBeNil)
	})

	Convey("Context cancels before sending the message endpoint", t, func() {

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		}))

		cl := NewSSE(ts.URL, nil)

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()

		pipe, err := cl.Start(ctx)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "did not receive /message endpoint in time: context deadline exceeded")
		So(pipe, ShouldBeNil)
	})

	Convey("Server does not respond with a valid status code", t, func() {

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusInsufficientStorage)
		}))

		cl := NewSSE(ts.URL, nil)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		pipe, err := cl.Start(ctx)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "invalid response from sse initialization: 507 Insufficient Storage")
		So(pipe, ShouldBeNil)
	})

	Convey("Server returns an invalid message message", t, func() {

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("this is not sse\n\n"))
		}))

		cl := NewSSE(ts.URL, nil)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		pipe, err := cl.Start(ctx)
		So(err, ShouldNotBeNil)
		So(pipe, ShouldBeNil)
		So(err.Error(), ShouldEqual, "unable to process sse message: invalid sse message: this is not sse")
	})

	Convey("Server returns a valid message endpoint", t, func() {

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("event: endpoint\ndata: /message?coucou=1\n\n"))
		}))

		cl := NewSSE(ts.URL, nil)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		pipe, err := cl.Start(ctx)
		So(err, ShouldBeNil)
		So(pipe, ShouldNotBeNil)
		So(cl.(*sseClient).messageEndpoint, ShouldEqual, fmt.Sprintf("%s/message?coucou=1", ts.URL))
	})

	Convey("Client sends a message and server replies correctly", t, func() {

		mutex := &sync.RWMutex{}

		wait := make(chan struct{}, 2)
		var rc *http.ResponseController
		var rw http.ResponseWriter
		var receivedMessage []byte

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

			if strings.HasSuffix(req.URL.String(), "/sse") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("event: endpoint\ndata: /message?coucou=1\n\n"))

				mutex.Lock()
				rc = http.NewResponseController(w)
				rw = w
				_ = rc.Flush()
				mutex.Unlock()

				<-req.Context().Done()
			}

			mutex.RLock()
			receivedMessage, _ = io.ReadAll(req.Body)
			w.WriteHeader(http.StatusAccepted)
			_, _ = rw.Write([]byte("event: message\ndata: coucou\n\n"))
			_ = rc.Flush()
			mutex.RLock()

			wait <- struct{}{}
		}))

		cl := NewSSE(ts.URL, nil)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		pipe, err := cl.Start(ctx)
		So(err, ShouldBeNil)
		So(pipe, ShouldNotBeNil)
		So(cl.(*sseClient).messageEndpoint, ShouldEqual, fmt.Sprintf("%s/message?coucou=1", ts.URL))

		pipe.Stdin <- []byte("this is a message")

		<-wait

		mutex.RLock()
		So(string(receivedMessage), ShouldEqual, "this is a message\n\n")
		mutex.RUnlock()

		So(string(<-pipe.Stdout), ShouldEqual, "coucou")
	})

	Convey("Client sends a message and server replies with invalid sse message", t, func() {

		mutex := &sync.RWMutex{}

		wait := make(chan struct{})
		var rc *http.ResponseController
		var rw http.ResponseWriter

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

			if strings.HasSuffix(req.URL.String(), "/sse") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("event: endpoint\ndata: /message?coucou=1\n\n"))

				mutex.Lock()
				rc = http.NewResponseController(w)
				rw = w
				_ = rc.Flush()
				mutex.Unlock()

				<-req.Context().Done()
			}

			mutex.RLock()
			w.WriteHeader(http.StatusAccepted)
			_, _ = rw.Write([]byte("this is not sse\n\n"))
			_ = rc.Flush()
			mutex.RUnlock()

			wait <- struct{}{}
		}))

		cl := NewSSE(ts.URL, nil)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		pipe, err := cl.Start(ctx)
		So(err, ShouldBeNil)
		So(pipe, ShouldNotBeNil)
		So(cl.(*sseClient).messageEndpoint, ShouldEqual, fmt.Sprintf("%s/message?coucou=1", ts.URL))

		pipe.Stdin <- []byte("this is a message")

		<-wait
		So((<-pipe.Exit).Error(), ShouldEqual, "invalid sse message: this is not sse")
	})

	Convey("Client sends a message and server replies with invalid status code", t, func() {

		wait := make(chan struct{})

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

			if strings.HasSuffix(req.URL.String(), "/sse") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("event: endpoint\ndata: /message?coucou=1\n\n"))
				rc := http.NewResponseController(w)
				_ = rc.Flush()
				<-req.Context().Done()
			}

			w.WriteHeader(http.StatusNotAcceptable)
			wait <- struct{}{}
		}))

		cl := NewSSE(ts.URL, nil)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		pipe, err := cl.Start(ctx)
		So(err, ShouldBeNil)
		So(pipe, ShouldNotBeNil)
		So(cl.(*sseClient).messageEndpoint, ShouldEqual, fmt.Sprintf("%s/message?coucou=1", ts.URL))

		pipe.Stdin <- []byte("this is a message")

		<-wait
		So((<-pipe.Exit).Error(), ShouldEqual, "invalid mcp server response status: 406 Not Acceptable")
	})

	Convey("Client sends a message but message endpoint is wrong", t, func() {

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

			if strings.HasSuffix(req.URL.String(), "/sse") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("event: endpoint\ndata: /message?coucou=1\n\n"))
				rc := http.NewResponseController(w)
				_ = rc.Flush()
				<-req.Context().Done()
			}
		}))

		cl := NewSSE(ts.URL, nil)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		pipe, err := cl.Start(ctx)
		So(err, ShouldBeNil)
		So(pipe, ShouldNotBeNil)
		cl.(*sseClient).messageEndpoint = "oh-no://999.999.999.999"

		pipe.Stdin <- []byte("this is a message")

		So((<-pipe.Exit).Error(), ShouldEqual, "unable to send post request: Post \"oh-no://999.999.999.999\": unsupported protocol scheme \"oh-no\"")
	})

	Convey("Client sends a message but message endpoint is not parseable", t, func() {

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

			if strings.HasSuffix(req.URL.String(), "/sse") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("event: endpoint\ndata: /message?coucou=1\n\n"))
				rc := http.NewResponseController(w)
				_ = rc.Flush()
				<-req.Context().Done()
			}
		}))

		cl := NewSSE(ts.URL, nil)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		pipe, err := cl.Start(ctx)
		So(err, ShouldBeNil)
		So(pipe, ShouldNotBeNil)
		cl.(*sseClient).messageEndpoint = "oh-no   :::::/999.999.999.999"

		pipe.Stdin <- []byte("this is a message")

		So((<-pipe.Exit).Error(), ShouldEqual, "unable to make post request: parse \"oh-no   :::::/999.999.999.999\": first path segment in URL cannot contain colon")
	})

	Convey("Client exits when server stops the connection", t, func() {

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

			if strings.HasSuffix(req.URL.String(), "/sse") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("event: endpoint\ndata: /message?coucou=1\n\n"))
				time.Sleep(time.Second)
			}
		}))

		cl := NewSSE(ts.URL, nil)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		pipe, err := cl.Start(ctx)
		So(err, ShouldBeNil)
		So(pipe, ShouldNotBeNil)

		So((<-pipe.Exit).Error(), ShouldEqual, "sse stream closed: EOF")
	})

}
