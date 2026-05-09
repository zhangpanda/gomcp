package adapter

import "testing"

// Verify toSnakeCase handles acronym runs without inserting an
// underscore between consecutive upper-case letters. Before the
// Batch-C fix "HTTPServer" became "h_t_t_p_server".
func TestToSnakeCase_Acronyms(t *testing.T) {
	cases := map[string]string{
		"UserService":  "user_service",
		"HTTPServer":   "http_server",
		"XMLParser":    "xml_parser",
		"GetUser":      "get_user",
		"ID":           "id",
		"GetID":        "get_id",
		"id":           "id",
		"APIKeyAuth":   "api_key_auth",
		"user_service": "user_service",
	}
	for in, want := range cases {
		if got := toSnakeCase(in); got != want {
			t.Errorf("toSnakeCase(%q) = %q, want %q", in, got, want)
		}
	}
}
