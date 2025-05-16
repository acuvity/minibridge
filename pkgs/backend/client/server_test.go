package client

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMCPServer(t *testing.T) {

	Convey("calling NewMCPServer on existing bin should work", t, func() {
		srv, err := NewMCPServer("echo", "hello")
		So(err, ShouldBeNil)
		So(srv.Command, ShouldEndWith, "/bin/echo")
		So(srv.Args, ShouldResemble, []string{"hello"})
	})

	Convey("calling NewMCPServer on non exiting bin should work", t, func() {
		_, err := NewMCPServer("not-echo", "hello")
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, `unable to find server binary: exec: "not-echo": executable file not found in $PATH`)
	})
}
