package mcp

import (
	"context"
	"encoding/json"
)

func (s *Server) registerResources() {
	s.resources["vessel://apps"] = s.resourceApps
	s.resources["vessel://health"] = s.resourceHealth
	s.resources["vessel://system"] = s.resourceSystem
}

func (s *Server) resourceApps(ctx context.Context) (string, error) {
	var apps []interface{}
	if err := s.client.Call("apps.list", nil, &apps); err != nil {
		return "", err
	}
	data, _ := json.MarshalIndent(apps, "", "  ")
	return string(data), nil
}

func (s *Server) resourceHealth(ctx context.Context) (string, error) {
	var health map[string]interface{}
	if err := s.client.Call("health.all", nil, &health); err != nil {
		return "", err
	}
	data, _ := json.MarshalIndent(health, "", "  ")
	return string(data), nil
}

func (s *Server) resourceSystem(ctx context.Context) (string, error) {
	info, err := s.toolSystemInfo(ctx, nil)
	if err != nil {
		return "", err
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	return string(data), nil
}
