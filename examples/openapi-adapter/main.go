package main

import (
	"log"

	"github.com/istarshine/gomcp"
	"github.com/istarshine/gomcp/adapter"
)

func main() {
	s := gomcp.New("openapi-demo", "1.0.0")

	err := adapter.ImportOpenAPI(s, "examples/openapi-adapter/petstore.yaml", adapter.OpenAPIOptions{
		TagFilter: []string{"pets"}, // only import "pets" tagged operations
		ServerURL: "https://petstore.example.com",
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Starting OpenAPI adapter MCP server...")
	if err := s.Stdio(); err != nil {
		log.Fatal(err)
	}
}
