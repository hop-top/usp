// Package mcp adapts the USP application API to a small MCP stdio server.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/internal/api"
)

const protocolVersion = "2024-11-05"

type Server struct {
	service *api.Service
}

func New(service *api.Service) *Server {
	if service == nil {
		service = api.NewDefault()
	}
	return &Server{service: service}
}

func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(w)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		resp, ok := s.Handle(ctx, line)
		if !ok {
			continue
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("mcp read: %w", err)
	}
	return nil
}

func (s *Server) Handle(ctx context.Context, raw []byte) (any, bool) {
	var req request
	if err := json.Unmarshal(raw, &req); err != nil {
		return errorResponse(nil, -32700, "parse error", nil), true
	}
	if req.Method == "" {
		return errorResponse(req.idPtr(), -32600, "invalid request", nil), true
	}
	if !req.hasID() && strings.HasPrefix(req.Method, "notifications/") {
		return nil, false
	}

	switch req.Method {
	case "initialize":
		return success(req.idPtr(), map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "usp", "version": "dev"},
		}), true
	case "ping":
		return success(req.idPtr(), map[string]any{}), true
	case "tools/list":
		return success(req.idPtr(), map[string]any{"tools": tools()}), true
	case "tools/call":
		result, err := s.callTool(ctx, req.Params)
		if err != nil {
			return errorResponse(req.idPtr(), -32602, err.Error(), nil), true
		}
		return success(req.idPtr(), result), true
	default:
		return errorResponse(req.idPtr(), -32601, "method not found", nil), true
	}
}

func (s *Server) callTool(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var p callParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid tools/call params: %w", err)
	}
	args := p.Arguments
	if args == nil {
		args = map[string]any{}
	}

	var payload any
	var err error
	switch p.Name {
	case "usp_session_list":
		payload, err = s.service.ListSessions(ctx, api.ListSessionsRequest{
			Project: stringArg(args, "project"),
			Tool:    stringArg(args, "tool"),
			Since:   sinceArg(args, "since"),
			Limit:   intArg(args, "limit", 20),
		})
	case "usp_session_search":
		query := stringArg(args, "query")
		if query == "" {
			return nil, fmt.Errorf("query is required")
		}
		payload, err = s.service.SearchSessions(ctx, api.SearchSessionsRequest{
			Project: stringArg(args, "project"),
			Tool:    stringArg(args, "tool"),
			Query:   query,
			Since:   sinceArg(args, "since"),
			Limit:   intArg(args, "limit", 20),
		})
	case "usp_session_show":
		id := stringArg(args, "id")
		if id == "" {
			return nil, fmt.Errorf("id is required")
		}
		payload, err = s.service.ShowSession(ctx, api.ShowSessionRequest{
			ID:            id,
			Tool:          stringArg(args, "tool"),
			Project:       stringArg(args, "project"),
			Since:         sinceArg(args, "since"),
			IncludeSkills: boolArg(args, "include_skills"),
		})
	case "usp_session_skills":
		payload, err = s.service.ListSkillEvents(ctx, api.ListSkillEventsRequest{
			SessionID: stringArg(args, "session"),
			Tool:      stringArg(args, "tool"),
			Project:   stringArg(args, "project"),
			Name:      stringArg(args, "name"),
			Since:     sinceArg(args, "since"),
			Until:     sinceArg(args, "until"),
		})
	default:
		return nil, fmt.Errorf("unknown tool %q", p.Name)
	}
	if err != nil {
		return toolError(err), nil
	}
	return toolText(payload), nil
}

func toolText(v any) map[string]any {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return toolError(err)
	}
	return map[string]any{
		"content": []map[string]string{{"type": "text", "text": string(raw)}},
	}
}

func toolError(err error) map[string]any {
	return map[string]any{
		"isError": true,
		"content": []map[string]string{{
			"type": "text",
			"text": err.Error(),
		}},
	}
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (r request) hasID() bool {
	return len(bytes.TrimSpace(r.ID)) > 0
}

func (r request) idPtr() *json.RawMessage {
	if !r.hasID() {
		return nil
	}
	id := append(json.RawMessage(nil), r.ID...)
	return &id
}

type callParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type response struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  any              `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func success(id *json.RawMessage, result any) response {
	return response{JSONRPC: "2.0", ID: id, Result: result}
}

func errorResponse(id *json.RawMessage, code int, message string, data any) response {
	return response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message, Data: data},
	}
}

func stringArg(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

func intArg(args map[string]any, key string, def int) int {
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case string:
		n, err := strconv.Atoi(x)
		if err == nil {
			return n
		}
	}
	return def
}

func boolArg(args map[string]any, key string) bool {
	v, ok := args[key]
	if !ok || v == nil {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		b, _ := strconv.ParseBool(x)
		return b
	default:
		return false
	}
}

func sinceArg(args map[string]any, key string) time.Time {
	t, err := sessionutil.ParseSince(stringArg(args, key))
	if err != nil {
		return time.Time{}
	}
	return t
}
