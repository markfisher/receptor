package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/receptor/handlers"
	"github.com/cloudfoundry-incubator/receptor/handlers/handler_fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Middleware", func() {
	var handler http.Handler
	var wrappedHandler *handler_fakes.FakeHandler
	var req *http.Request
	var res *httptest.ResponseRecorder

	BeforeEach(func() {
		req = newTestRequest("")
		res = httptest.NewRecorder()
		wrappedHandler = new(handler_fakes.FakeHandler)
	})

	Describe("CORSWrapper", func() {
		JustBeforeEach(func() {
			handler = handlers.CORSWrapper(wrappedHandler)
			handler.ServeHTTP(res, req)
		})

		Context("when the Origin header on the request is valid", func() {
			const expectedOrigin = "example.com"

			BeforeEach(func() {
				req.Header.Set("Origin", expectedOrigin)
			})

			It("calls the wrapped handler", func() {
				Ω(wrappedHandler.ServeHTTPCallCount()).Should(Equal(1))
			})

			It("sets the CORS response headers", func() {
				headers := res.Header()
				Ω(headers.Get("Access-Control-Allow-Origin")).Should(Equal(expectedOrigin))
				Ω(headers.Get("Access-Control-Allow-Credentials")).Should(Equal("true"))
			})
		})

		Context("when the Origin header on the request is blacklisted", func() {
			invalidOriginHeaders := []string{"", "*"}

			for _, invalidHeader := range invalidOriginHeaders {
				Context(fmt.Sprint("such as '", invalidHeader, "'"), func() {
					BeforeEach(func() {
						req.Header.Set("Origin", invalidHeader)
					})

					It("calls the wrapped handler", func() {
						Ω(wrappedHandler.ServeHTTPCallCount()).Should(Equal(1))
					})

					It("does not set the CORS response headers", func() {
						_, isSet := res.Header()["Access-Control-Allow-Origin"]
						Ω(isSet).Should(BeFalse())

						_, isSet = res.Header()["Access-Control-Allow-Credentials"]
						Ω(isSet).Should(BeFalse())
					})
				})
			}
		})

		Context("when a valid CORS preflight request is made", func() {
			const (
				expectedOrigin         = "example.com"
				expectedAllowedMethods = "PUT"
				expectedAllowedHeaders = "content-type,authorization"
			)

			BeforeEach(func() {
				req.Method = "OPTIONS"
				req.Header.Set("Origin", expectedOrigin)
				req.Header.Set("Access-Control-Request-Method", expectedAllowedMethods)
				req.Header.Set("Access-Control-Request-Headers", expectedAllowedHeaders)
			})

			It("does not call the wrapped handler", func() {
				Ω(wrappedHandler.ServeHTTPCallCount()).Should(Equal(0))
			})

			It("responds with 200 OK", func() {
				Ω(res.Code).Should(Equal(http.StatusOK))
			})

			It("sets the CORS preflight response headers", func() {
				headers := res.Header()
				Ω(headers.Get("Access-Control-Allow-Origin")).Should(Equal(expectedOrigin))
				Ω(headers.Get("Access-Control-Allow-Credentials")).Should(Equal("true"))
				Ω(headers.Get("Access-Control-Allow-Methods")).Should(Equal(expectedAllowedMethods))
				Ω(headers.Get("Access-Control-Allow-Headers")).Should(Equal(expectedAllowedHeaders))
			})
		})
	})

	Describe("CookieAuthWrap", func() {
		var cookieName string

		BeforeEach(func() {
			cookieName = "Cookie-Authorization"
			handler = handlers.CookieAuthWrap(wrappedHandler, cookieName)
		})

		Context("when the cookie is present in the request", func() {
			BeforeEach(func() {
				req.AddCookie(&http.Cookie{
					Name:  cookieName,
					Value: "some-auth",
				})

				req.Header.Add("Authorization", "some-clobbered-auth")

				handler.ServeHTTP(res, req)
			})

			It("forwards it as the request's Authorization header", func() {
				Ω(wrappedHandler.ServeHTTPCallCount()).Should(Equal(1))

				_, wrappedReq := wrappedHandler.ServeHTTPArgsForCall(0)
				Ω(wrappedReq.Header.Get("Authorization")).Should(Equal("some-auth"))
			})
		})

		Context("when the cookie is not present in the request", func() {
			BeforeEach(func() {
				req.Header.Add("Authorization", "some-not-clobbered-auth")
				handler.ServeHTTP(res, req)
			})

			It("does not clobber the Authorization header", func() {
				Ω(wrappedHandler.ServeHTTPCallCount()).Should(Equal(1))

				_, wrappedReq := wrappedHandler.ServeHTTPArgsForCall(0)
				Ω(wrappedReq.Header.Get("Authorization")).Should(Equal("some-not-clobbered-auth"))
			})
		})
	})

	Describe("BasicAuthWrap", func() {
		var expectedUsername = "user"
		var expectedPassword = "pass"

		BeforeEach(func() {
			handler = handlers.BasicAuthWrap(wrappedHandler, expectedUsername, expectedPassword)
		})

		Context("when the correct credentials are provided", func() {
			BeforeEach(func() {
				req.SetBasicAuth(expectedUsername, expectedPassword)
				handler.ServeHTTP(res, req)
			})

			It("calls the wrapped handler", func() {
				Ω(wrappedHandler.ServeHTTPCallCount()).Should(Equal(1))
			})
		})

		Context("when no credentials are provided", func() {
			BeforeEach(func() {
				handler.ServeHTTP(res, req)
			})

			It("doesn't call the wrapped handler", func() {
				Ω(wrappedHandler.ServeHTTPCallCount()).Should(Equal(0))
			})
		})

		Context("when incorrect credentials are provided", func() {
			BeforeEach(func() {
				req.SetBasicAuth(expectedUsername, "badPassword")
				handler.ServeHTTP(res, req)
			})

			It("returns 401 UNAUTHORIZED", func() {
				Ω(res.Code).Should(Equal(http.StatusUnauthorized))
			})

			It("returns an unauthorized error response", func() {
				expectedBody, _ := json.Marshal(receptor.Error{
					Type:    receptor.Unauthorized,
					Message: http.StatusText(http.StatusUnauthorized),
				})
				Ω(res.Body.String()).Should(Equal(string(expectedBody)))
			})

			It("doesn't call the wrapped handler", func() {
				Ω(wrappedHandler.ServeHTTPCallCount()).Should(Equal(0))
			})
		})
	})
})
