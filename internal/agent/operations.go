package agent

import (
	"context"
	"encoding/json"
)

func RegisterBuiltinOps(s *Server) {
	s.Register("ping", opPing)
}

func opPing(_ context.Context, _ json.RawMessage) (any, error) {
	return map[string]string{"status": "pong"}, nil
}
