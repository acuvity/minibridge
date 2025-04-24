package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"go.acuvity.ai/bahamut"
)

func TestThing(t *testing.T) {

	Convey("Given I have a response writer, req and corsPolicy", t, func() {
		w := httptest.NewRecorder()
		req := &http.Request{}
		pol := &bahamut.CORSPolicy{
			AllowOrigin:      "https://coucou.test",
			AllowCredentials: true,
			MaxAge:           1500,
			AllowHeaders: []string{
				"Authorization",
			},
			AllowMethods: []string{
				"GET",
				"POST",
				"OPTIONS",
			},
		}

		Convey("Calling HandleCors should work on OPTIONS", func() {

			req.Method = http.MethodOptions

			shouldCont := HandleCORS(w, req, pol)
			So(shouldCont, ShouldBeFalse)
			So(w.Result().Header, ShouldResemble, http.Header{
				"Access-Control-Allow-Credentials": {"true"},
				"Access-Control-Allow-Headers":     {"Authorization"},
				"Access-Control-Allow-Methods":     {"GET, POST, OPTIONS"},
				"Access-Control-Allow-Origin":      {"https://coucou.test"},
				"Access-Control-Expose-Headers":    {""},
				"Access-Control-Max-Age":           {"1500"},
				"Cache-Control":                    {"private, no-transform"},
				"Strict-Transport-Security":        {"max-age=31536000; includeSubDomains; preload"},
				"X-Content-Type-Options":           {"nosniff"},
				"X-Frame-Options":                  {"DENY"},
				"X-Xss-Protection":                 {"1; mode=block"},
			})
		})

		Convey("Calling HandleCors should work on non OPTIONS", func() {

			req.Method = http.MethodPost

			shouldCont := HandleCORS(w, req, pol)
			So(shouldCont, ShouldBeTrue)
			So(w.Result().Header, ShouldResemble, http.Header{
				"Access-Control-Allow-Credentials": {"true"},
				"Access-Control-Allow-Origin":      {"https://coucou.test"},
				"Access-Control-Expose-Headers":    {""},
				"Cache-Control":                    {"private, no-transform"},
				"Strict-Transport-Security":        {"max-age=31536000; includeSubDomains; preload"},
				"X-Content-Type-Options":           {"nosniff"},
				"X-Frame-Options":                  {"DENY"},
				"X-Xss-Protection":                 {"1; mode=block"},
			})
		})

		Convey("Calling HandleCors should work with no policy", func() {

			req.Method = http.MethodPost

			shouldCont := HandleCORS(w, req, nil)
			So(shouldCont, ShouldBeTrue)
			So(w.Result().Header, ShouldResemble, http.Header{
				"Cache-Control":             {"private, no-transform"},
				"Strict-Transport-Security": {"max-age=31536000; includeSubDomains; preload"},
				"X-Content-Type-Options":    {"nosniff"},
				"X-Frame-Options":           {"DENY"},
				"X-Xss-Protection":          {"1; mode=block"},
			})
		})
	})
}
