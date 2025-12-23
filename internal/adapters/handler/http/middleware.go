package http

import "net/http"

// SecureHeaders adds security headers to every response
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Prevent MIME-sniffing (Forces browser to respect Content-Type)
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// 2. Prevent clickjacking (Your API shouldn't be inside an iframe)
		w.Header().Set("X-Frame-Options", "DENY")

		// 3. XSS Protection (Legacy but good depth)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// 4. Content Security Policy (The most powerful header)
		// This tells the browser: "Only trust scripts from ME, no inline scripts allowed"
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; object-src 'none';")

		next.ServeHTTP(w, r)
	})
}
