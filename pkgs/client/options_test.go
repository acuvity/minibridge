package client

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestOptions(t *testing.T) {

	Convey("OptUseTempDir should work", t, func() {
		cfg := newCfg()
		OptUseTempDir(true)(&cfg)
		So(cfg.useTempDir, ShouldBeTrue)
	})

	Convey("OptCredentials should work", t, func() {
		cfg := newCfg()
		OptCredentials(1000, 1001, []int{2001, 2002})(&cfg)
		So(cfg.creds.Uid, ShouldEqual, 1000)
		So(cfg.creds.Gid, ShouldEqual, 1001)
		So(cfg.creds.Groups, ShouldResemble, []uint32{2001, 2002})
	})

	Convey("OptCredentials with -1 should work", t, func() {
		cfg := newCfg()
		OptCredentials(-1, -1, []int{-1})(&cfg)
		So(cfg.creds, ShouldBeNil)
	})

	Convey("OptCredentials with only gid -1 should work", t, func() {
		cfg := newCfg()
		OptCredentials(100, -1, []int{-1})(&cfg)
		So(cfg.creds.Uid, ShouldEqual, 100)
		So(cfg.creds.Gid, ShouldEqual, 0)
		So(cfg.creds.Groups, ShouldResemble, []uint32{})
	})

	Convey("OptCredentials with uint32 overflow on uid should fail", t, func() {
		cfg := newCfg()
		So(func() { OptCredentials(math.MaxInt64, 1001, []int{2001, 2002})(&cfg) }, ShouldPanicWith, "invalid uid. overflows")
	})

	Convey("OptCredentials with uint32 overflow on uid should fail", t, func() {
		cfg := newCfg()
		So(func() { OptCredentials(1, math.MaxInt64, []int{2001, 2002})(&cfg) }, ShouldPanicWith, "invalid gid. overflows")
	})

	Convey("OptCredentials with uint32 overflow on groups should fail", t, func() {
		cfg := newCfg()
		So(func() { OptCredentials(1, 1, []int{2001, math.MaxInt64})(&cfg) }, ShouldPanicWith, "invalid group 1. overflows")
	})
}
