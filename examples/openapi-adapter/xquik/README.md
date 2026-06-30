# Xquik OpenAPI Adapter Example

This example imports a small Xquik OpenAPI fixture and exposes the
`searchXquikTweets` operation as an MCP tool.

## Run

```bash
export XQUIK_BEARER_TOKEN="<your-xquik-bearer-token>"
go run ./examples/openapi-adapter/xquik
```

The fixture uses `https://xquik.com` as the server URL and keeps the example
read-only by importing the tweet search endpoint.
