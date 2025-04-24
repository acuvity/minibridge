package backend

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
)

func TestCarrier(t *testing.T) {

	Convey("MCPCarrier from call with no params", t, func() {
		c := newMCPMetaCarrier(api.NewMCPCall(1))
		So(len(c.meta), ShouldEqual, 0)
		So(c.Get("a"), ShouldBeEmpty)
		c.Set("a", "1")
		So(c.Get("a"), ShouldEqual, "1")
		So(c.Keys(), ShouldResemble, []string{"a"})
	})

	Convey("MCPCarrier from call with params but no _meta", t, func() {
		call := api.NewMCPCall(1)
		call.Params = map[string]any{}
		c := newMCPMetaCarrier(call)
		So(len(c.meta), ShouldEqual, 0)
	})

	Convey("MCPCarrier from call with params with _meta with wrong type", t, func() {
		call := api.NewMCPCall(1)
		call.Params = map[string]any{"_meta": "oh no"}
		c := newMCPMetaCarrier(call)
		So(len(c.meta), ShouldEqual, 0)
	})

	Convey("MCPCarrier from call with params with wrong _meta value type", t, func() {
		call := api.NewMCPCall(1)
		call.Params = map[string]any{"_meta": map[string]any{"a": 42}}
		c := newMCPMetaCarrier(call)
		So(len(c.meta), ShouldEqual, 0)
	})

	Convey("MCPCarrier from call with valid _meta", t, func() {
		call := api.NewMCPCall(1)
		call.Params = map[string]any{"_meta": map[string]any{"a": "42"}}
		c := newMCPMetaCarrier(call)
		So(len(c.meta), ShouldEqual, 1)
		So(c.Get("a"), ShouldEqual, "42")
		So(c.Keys(), ShouldResemble, []string{"a"})
		So(call.Params["_meta"], ShouldBeNil)
	})

}
