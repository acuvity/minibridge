package backend

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"go.acuvity.ai/minibridge/pkgs/mcp"
)

func TestCarrier(t *testing.T) {

	Convey("MCPCarrier from call with no params", t, func() {
		msg := newMCPMetaCarrier(mcp.NewMessage(1))
		So(len(msg.meta), ShouldEqual, 0)
		So(msg.Get("a"), ShouldBeEmpty)
		msg.Set("a", "1")
		So(msg.Get("a"), ShouldEqual, "1")
		So(msg.Keys(), ShouldResemble, []string{"a"})
	})

	Convey("MCPCarrier from call with params but no _meta", t, func() {
		msg := mcp.NewMessage(1)
		msg.Params = map[string]any{}
		c := newMCPMetaCarrier(msg)
		So(len(c.meta), ShouldEqual, 0)
	})

	Convey("MCPCarrier from call with params with _meta with wrong type", t, func() {
		msg := mcp.NewMessage(1)
		msg.Params = map[string]any{"_meta": "oh no"}
		c := newMCPMetaCarrier(msg)
		So(len(c.meta), ShouldEqual, 0)
	})

	Convey("MCPCarrier from call with params with wrong _meta value type", t, func() {
		msg := mcp.NewMessage(1)
		msg.Params = map[string]any{"_meta": map[string]any{"a": 42}}
		c := newMCPMetaCarrier(msg)
		So(len(c.meta), ShouldEqual, 0)
	})

	Convey("MCPCarrier from call with valid _meta", t, func() {
		msg := mcp.NewMessage(1)
		msg.Params = map[string]any{"_meta": map[string]any{"a": "42"}}
		c := newMCPMetaCarrier(msg)
		So(len(c.meta), ShouldEqual, 1)
		So(c.Get("a"), ShouldEqual, "42")
		So(c.Keys(), ShouldResemble, []string{"a"})
		So(msg.Params["_meta"], ShouldNotBeNil)
	})

}
