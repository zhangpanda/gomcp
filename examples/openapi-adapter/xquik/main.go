package main

import (
	"log"
	"os"

	"github.com/zhangpanda/gomcp"
	"github.com/zhangpanda/gomcp/adapter"
)

func main() {
	authToken := os.Getenv("XQUIK_BEARER_TOKEN")
	if authToken == "" {
		log.Fatal("set XQUIK_BEARER_TOKEN before starting the Xquik example")
	}

	s := gomcp.New("xquik-openapi", "1.0.0")
	err := adapter.ImportOpenAPI(s, "examples/openapi-adapter/xquik/xquik.openapi.yaml", adapter.OpenAPIOptions{
		AuthToken: authToken,
		ServerURL: "https://xquik.com",
		TagFilter: []string{"xquik"},
	})
	if err != nil {
		log.Fatal(err)
	}

	s.Stdio()
}
