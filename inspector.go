package gomcp

import "github.com/zhangpanda/gomcp/inspector"

// Dev starts the server in development mode with Inspector UI on the given address.
func (s *Server) Dev(addr string) error {
	s.logger.Info("starting MCP dev server with Inspector", "addr", addr)
	return inspector.Dev(s, addr)
}
