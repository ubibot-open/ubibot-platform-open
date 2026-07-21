package api

import (
	"encoding/json"
	"net/http"
)

// writeJSON and decodeJSON are the two helpers every handler in this
// package uses instead of a web framework's binding/rendering — the
// device and admin APIs here are small enough that net/http's own
// ServeMux (Go 1.22+ method+pattern routing) covers routing needs
// without pulling in a framework and its transitive dependency tree.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}
