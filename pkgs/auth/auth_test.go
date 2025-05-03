package auth

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestAuth(t *testing.T) {

	Convey("Basic auth should work", t, func() {
		auth := NewBasicAuth("user", "pass")
		So(auth.Type(), ShouldEqual, "Basic")
		So(auth.User(), ShouldEqual, "user")
		So(auth.Password(), ShouldEqual, "pass")
		So(auth.Encode(), ShouldEqual, "Basic dXNlcjpwYXNz")
	})

	Convey("Bearer auth should work", t, func() {
		auth := NewBearerAuth("token")
		So(auth.Type(), ShouldEqual, "Bearer")
		So(auth.User(), ShouldEqual, "Bearer")
		So(auth.Password(), ShouldEqual, "token")
		So(auth.Encode(), ShouldEqual, "Bearer token")
	})
}
