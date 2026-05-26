package httpapi

import "net/http"

func Method(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			Error(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		next(w, r)
	}
}
