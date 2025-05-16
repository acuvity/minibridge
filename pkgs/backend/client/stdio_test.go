package client

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStdioClient(t *testing.T) {

	Convey("Type is correct", t, func() {
		cl := NewStdio(MCPServer{})
		So(cl.Type(), ShouldEqual, "stdio")
	})

	Convey("Given I have a client cat and I send lots of trailing \n", t, func() {

		srv := MCPServer{
			Command: "cat",
			Env:     []string{"A=A"},
		}
		cl := NewStdio(srv)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		stream, err := cl.Start(ctx)
		So(err, ShouldBeNil)
		So(stream, ShouldNotBeNil)

		out, unregister := stream.Stdout()
		defer unregister()

		stream.Stdin() <- []byte("hello world\r\n\n\n")

		data := <-out
		So(data, ShouldResemble, []byte("hello world"))
	})

	Convey("Given I have a client with env", t, func() {

		srv := MCPServer{
			Command: "sh",
			Args:    []string{"-c", "echo $MTEST"},
			Env:     []string{"MTEST=HELLO"},
		}
		cl := NewStdio(srv)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		stream, err := cl.Start(ctx)
		So(err, ShouldBeNil)
		So(stream, ShouldNotBeNil)

		out, unregisterOut := stream.Stdout()
		defer unregisterOut()

		exit, unregisterExit := stream.Exit()
		defer unregisterExit()

		data := <-out
		So(string(data), ShouldEqual, "HELLO")

		err = <-exit
		So(err, ShouldBeNil)
	})

	Convey("Given I have a client to which I give an invalid server", t, func() {

		srv := MCPServer{
			Command: "dog",
		}
		cl := NewStdio(srv)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		stream, err := cl.Start(ctx)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, `unable to start command: exec: "dog": executable file not found in $PATH`)
		So(stream, ShouldBeNil)
	})

	Convey("Given I have a client with a command that exits unexpectedly", t, func() {

		srv := MCPServer{
			Command: "bash",
			Args:    []string{"-c", "sleep 1 && exit 1"},
		}
		cl := NewStdio(srv)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		stream, err := cl.Start(ctx)
		So(err, ShouldBeNil)
		So(stream, ShouldNotBeNil)

		exit, unregister := stream.Exit()
		defer unregister()

		time.Sleep(1050 * time.Millisecond)

		err = <-exit
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "exit status 1")
	})

	Convey("Given I have a client that writes a file", t, func() {

		srv := MCPServer{
			Command: "sh",
			Args:    []string{"-c", "touch testfile"},
		}
		cl := NewStdio(srv)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		stream, err := cl.Start(ctx)
		So(err, ShouldBeNil)
		So(stream, ShouldNotBeNil)

		exit, unregister := stream.Exit()
		defer unregister()

		So(<-exit, ShouldBeNil)

		_, err = os.Stat("testfile")
		So(err, ShouldBeNil)
		_ = os.RemoveAll("testfile")
	})

	Convey("Given I have a client that writes a file with tempdir", t, func() {

		srv := MCPServer{
			Command: "sh",
			Args:    []string{"-c", "touch testfile"},
		}
		cl := NewStdio(srv, OptStdioUseTempDir(true))

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		stream, err := cl.Start(ctx)
		So(err, ShouldBeNil)
		So(stream, ShouldNotBeNil)

		exit, unregister := stream.Exit()
		defer unregister()

		So(<-exit, ShouldBeNil)

		_, err = os.Stat("testfile")
		So(err.Error(), ShouldEqual, "stat testfile: no such file or directory")
	})

	Convey("Given I have a running client and an expiring context", t, func() {

		srv := MCPServer{
			Command: "cat",
		}
		cl := NewStdio(srv)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		stream, err := cl.Start(ctx)
		So(err, ShouldBeNil)
		So(stream, ShouldNotBeNil)

		exit, unregister := stream.Exit()
		defer unregister()

		time.Sleep(300 * time.Millisecond)
		cancel()

		err = <-exit
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "signal: interrupt")
	})
}
