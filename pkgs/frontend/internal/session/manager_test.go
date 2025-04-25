package session

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"go.acuvity.ai/wsc"
)

func TestManager(t *testing.T) {

	Convey("Manager should work", t, func() {

		ws1 := wsc.NewMockWebsocket(t.Context())
		ws2 := wsc.NewMockWebsocket(t.Context())

		m := NewManager()
		s1 := New(ws1, 1, "1")
		s2 := newSession(ws2, 2, "2", 2*time.Second)

		So(s1.nextDeadline, ShouldEqual, defaultDeadlineDuration)
		So(s2.nextDeadline, ShouldEqual, 2*time.Second)
		So(s1.ValidateHash(1), ShouldBeTrue)
		So(s2.ValidateHash(2), ShouldBeTrue)

		m.Register(s1)
		m.Register(s2)
		So(len(m.sessions), ShouldEqual, 2)
		So(m.sessions["1"].getCount(), ShouldEqual, 1)
		So(m.sessions["2"].getCount(), ShouldEqual, 1)

		// We acquire 1
		m.Acquire("1", nil)
		So(len(m.sessions), ShouldEqual, 2)
		So(m.sessions["1"].getCount(), ShouldEqual, 2)
		So(m.sessions["2"].getCount(), ShouldEqual, 1)

		go func() { s1.Write([]byte("coucou")) }()
		So(string(<-ws1.LastWrite()), ShouldEqual, "coucou")

		// We releease 1
		m.Release("1", nil)
		So(len(m.sessions), ShouldEqual, 2)
		So(m.sessions["1"].getCount(), ShouldEqual, 1)
		So(m.sessions["2"].getCount(), ShouldEqual, 1)

		// We release 1 again, it should be removed
		m.Release("1", nil)
		So(len(m.sessions), ShouldEqual, 1)
		So(m.sessions["1"], ShouldBeNil)
		So(m.sessions["2"].getCount(), ShouldEqual, 1)

		// we over release, it should be noop
		m.Release("1", nil)
		So(len(m.sessions), ShouldEqual, 1)
		So(m.sessions["1"], ShouldBeNil)
		So(m.sessions["2"].getCount(), ShouldEqual, 1)

		// We simulate a message when there is no hook
		// this should be noop and should not break the rest of
		// the test
		ws2.NextRead([]byte("nobody there"))
		<-time.After(time.Second)

		// We acquire 2 with a chan
		ch1 := make(chan []byte)
		ch2 := make(chan []byte)
		m.Acquire("2", ch1)
		m.Acquire("2", ch2)
		So(len(m.sessions), ShouldEqual, 1)
		So(m.sessions["2"].getCount(), ShouldEqual, 3)
		So(len(m.sessions["2"].getHooks()), ShouldEqual, 2)

		// we simulate a message from the ws
		ws2.NextRead([]byte("coucou"))

		// we should get it on ch1
		var data []byte
		select {
		case data = <-ch1:
		case <-time.After(time.Second):
		}
		So(string(data), ShouldEqual, "coucou")

		// we should get it on ch2
		select {
		case data = <-ch2:
		case <-time.After(time.Second):
		}
		So(string(data), ShouldEqual, "coucou")

		// Now we release 2 twice
		m.Release("2", ch1)
		m.Release("2", ch2)
		So(len(m.sessions), ShouldEqual, 1)
		So(m.sessions["2"].getCount(), ShouldEqual, 1)
		So(len(m.sessions["2"].getHooks()), ShouldEqual, 0)

		select {
		case <-s2.Done():
			m.Release("2", nil)
		case <-time.After(5 * time.Second):
		}

		So(len(m.sessions), ShouldEqual, 0)
	})
}
