package main

import (
	"crypto/subtle"
	"net/http"
)

// requireOriginHeader rejects requests that did not come through CloudFront.
// CloudFront adds X-Origin-Verify with a shared secret to every request it
// forwards; anything calling the execute-api URL directly lacks it and would
// otherwise bypass the CloudFront WAF and eat the stage's shared throttle.
// API Gateway REST resource policies cannot match custom headers, so the check
// has to live here.
//
// An empty secret disables the check, so local runs without CloudFront work.
// GET / is exempt because the Lambda Web Adapter readiness probe calls it
// without the header; in Lambda it only serves the empty SPA directory.
func requireOriginHeader(next http.Handler, secret string) http.Handler {
	if secret == "" {
		return next
	}
	want := []byte(secret)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Origin-Verify")), want) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
