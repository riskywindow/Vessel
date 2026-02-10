package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

func (s *Server) registerTools() {
	// Read-only tools (Session 1)
	s.tools["vessel_list_apps"] = s.toolListApps
	s.tools["vessel_get_app"] = s.toolGetApp
	s.tools["vessel_health"] = s.toolHealth
	s.tools["vessel_logs"] = s.toolLogs
	s.tools["vessel_metrics"] = s.toolMetrics
	s.tools["vessel_system_info"] = s.toolSystemInfo
	s.tools["vessel_deploy_history"] = s.toolDeployHistory

	// Action tools — stubs for Session 2
	s.tools["vessel_deploy"] = s.toolNotImplemented("vessel_deploy")
	s.tools["vessel_stop"] = s.toolNotImplemented("vessel_stop")
	s.tools["vessel_restart"] = s.toolNotImplemented("vessel_restart")
	s.tools["vessel_remove"] = s.toolNotImplemented("vessel_remove")
	s.tools["vessel_rollback"] = s.toolNotImplemented("vessel_rollback")
}

func (s *Server) toolNotImplemented(name string) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		return nil, fmt.Errorf("%s will be implemented in Session 2", name)
	}
}

// getToolDefinitions returns all tool definitions for tools/list.
func (s *Server) getToolDefinitions() []Tool {
	return []Tool{
		{
			Name:        "vessel_list_apps",
			Description: "List all deployed applications with their status, containers, and health",
			InputSchema: InputSchema{Type: "object"},
		},
		{
			Name:        "vessel_get_app",
			Description: "Get detailed information about a specific application",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name": {Type: "string", Description: "Application name"},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "vessel_deploy",
			Description: "Deploy a new application or update an existing one",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name":      {Type: "string", Description: "Application name"},
					"image":     {Type: "string", Description: "Container image (e.g., nginx:latest)"},
					"instances": {Type: "integer", Description: "Number of instances (default: 1)"},
					"command":   {Type: "string", Description: "Override command (optional)"},
					"hostname":  {Type: "string", Description: "Hostname for HTTP routing (optional)"},
				},
				Required: []string{"name", "image"},
			},
		},
		{
			Name:        "vessel_stop",
			Description: "Stop a running application",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name": {Type: "string", Description: "Application name"},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "vessel_restart",
			Description: "Restart an application",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name": {Type: "string", Description: "Application name"},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "vessel_remove",
			Description: "Remove an application completely",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name": {Type: "string", Description: "Application name"},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "vessel_rollback",
			Description: "Rollback an application to a previous version",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name":    {Type: "string", Description: "Application name"},
					"version": {Type: "integer", Description: "Version number to rollback to"},
				},
				Required: []string{"name", "version"},
			},
		},
		{
			Name:        "vessel_logs",
			Description: "Get recent logs from an application's containers",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name": {Type: "string", Description: "Application name"},
					"tail": {Type: "integer", Description: "Number of lines (default: 100)"},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "vessel_health",
			Description: "Get health status of containers",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name": {Type: "string", Description: "Application name (optional, omit for all)"},
				},
			},
		},
		{
			Name:        "vessel_metrics",
			Description: "Get resource usage metrics (CPU, memory) for an application",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name": {Type: "string", Description: "Application name"},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "vessel_system_info",
			Description: "Get Vessel system information including app and container counts",
			InputSchema: InputSchema{Type: "object"},
		},
		{
			Name:        "vessel_deploy_history",
			Description: "Get deployment history for an application",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name": {Type: "string", Description: "Application name"},
				},
				Required: []string{"name"},
			},
		},
	}
}

// Read-only tool implementations

func (s *Server) toolListApps(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var apps []interface{}
	if err := s.client.Call("apps.list", nil, &apps); err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	if len(apps) == 0 {
		return map[string]interface{}{
			"message": "No applications deployed",
			"apps":    []interface{}{},
		}, nil
	}

	return map[string]interface{}{
		"count": len(apps),
		"apps":  apps,
	}, nil
}

func (s *Server) toolGetApp(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	if params.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	var app interface{}
	if err := s.client.Call("apps.get", map[string]string{"name": params.Name}, &app); err != nil {
		return nil, fmt.Errorf("app %q not found: %w", params.Name, err)
	}

	return app, nil
}

func (s *Server) toolHealth(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Name string `json:"name"`
	}
	json.Unmarshal(args, &params) // Optional param, ignore error

	var health map[string]interface{}
	if err := s.client.Call("health.all", nil, &health); err != nil {
		return nil, fmt.Errorf("failed to get health: %w", err)
	}

	// If name specified, filter to that app's containers
	if params.Name != "" {
		var app struct {
			Containers []struct {
				ID string `json:"id"`
			} `json:"containers"`
		}
		if err := s.client.Call("apps.get", map[string]string{"name": params.Name}, &app); err != nil {
			return nil, fmt.Errorf("app %q not found: %w", params.Name, err)
		}

		filtered := make(map[string]interface{})
		for _, c := range app.Containers {
			if h, ok := health[c.ID]; ok {
				shortID := c.ID
				if len(shortID) > 12 {
					shortID = shortID[:12]
				}
				filtered[shortID] = h
			}
		}
		return map[string]interface{}{
			"app":    params.Name,
			"health": filtered,
		}, nil
	}

	return map[string]interface{}{"health": health}, nil
}

func (s *Server) toolLogs(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Name string `json:"name"`
		Tail int    `json:"tail"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	if params.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if params.Tail <= 0 {
		params.Tail = 100
	}

	// Get app to find containers
	var app struct {
		Containers []struct {
			ID    string `json:"id"`
			State string `json:"state"`
		} `json:"containers"`
	}
	if err := s.client.Call("apps.get", map[string]string{"name": params.Name}, &app); err != nil {
		return nil, fmt.Errorf("app %q not found: %w", params.Name, err)
	}

	// Find first running container
	var containerID string
	for _, c := range app.Containers {
		if c.State == "running" {
			containerID = c.ID
			break
		}
	}

	if containerID == "" {
		return map[string]interface{}{
			"app":     params.Name,
			"message": "No running containers",
			"logs":    "",
		}, nil
	}

	var logs struct {
		Logs string `json:"logs"`
	}
	if err := s.client.Call("containers.logs", map[string]interface{}{
		"id":   containerID,
		"tail": params.Tail,
	}, &logs); err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	shortID := containerID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	return map[string]interface{}{
		"app":          params.Name,
		"container_id": shortID,
		"logs":         logs.Logs,
	}, nil
}

func (s *Server) toolMetrics(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	if params.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	var app struct {
		Containers []struct {
			ID    string `json:"id"`
			State string `json:"state"`
		} `json:"containers"`
	}
	if err := s.client.Call("apps.get", map[string]string{"name": params.Name}, &app); err != nil {
		return nil, fmt.Errorf("app %q not found: %w", params.Name, err)
	}

	var metrics []map[string]interface{}
	for _, c := range app.Containers {
		if c.State != "running" {
			continue
		}

		var metric interface{}
		if err := s.client.Call("metrics.latest", map[string]string{"container_id": c.ID}, &metric); err == nil {
			shortID := c.ID
			if len(shortID) > 12 {
				shortID = shortID[:12]
			}
			metrics = append(metrics, map[string]interface{}{
				"container_id": shortID,
				"metrics":      metric,
			})
		}
	}

	if len(metrics) == 0 {
		return map[string]interface{}{
			"app":     params.Name,
			"message": "No metrics available",
			"metrics": []interface{}{},
		}, nil
	}

	return map[string]interface{}{
		"app":     params.Name,
		"metrics": metrics,
	}, nil
}

func (s *Server) toolSystemInfo(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var apps []interface{}
	s.client.Call("apps.list", nil, &apps)

	var health map[string]interface{}
	s.client.Call("health.all", nil, &health)

	healthyCount := 0
	unhealthyCount := 0
	for _, h := range health {
		if hm, ok := h.(map[string]interface{}); ok {
			if status, ok := hm["status"].(string); ok {
				if status == "healthy" {
					healthyCount++
				} else if status == "unhealthy" {
					unhealthyCount++
				}
			}
		}
	}

	runningContainers := 0
	totalContainers := 0
	for _, a := range apps {
		if am, ok := a.(map[string]interface{}); ok {
			if containers, ok := am["containers"].([]interface{}); ok {
				totalContainers += len(containers)
				for _, c := range containers {
					if cm, ok := c.(map[string]interface{}); ok {
						if state, ok := cm["state"].(string); ok && state == "running" {
							runningContainers++
						}
					}
				}
			}
		}
	}

	return map[string]interface{}{
		"version":              "0.1.0",
		"total_apps":           len(apps),
		"total_containers":     totalContainers,
		"running_containers":   runningContainers,
		"healthy_containers":   healthyCount,
		"unhealthy_containers": unhealthyCount,
	}, nil
}

func (s *Server) toolDeployHistory(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	if params.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	var deploys []interface{}
	if err := s.client.Call("apps.history", map[string]string{"name": params.Name}, &deploys); err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	if len(deploys) == 0 {
		return map[string]interface{}{
			"app":     params.Name,
			"message": "No deploy history",
			"deploys": []interface{}{},
		}, nil
	}

	return map[string]interface{}{
		"app":     params.Name,
		"count":   len(deploys),
		"deploys": deploys,
	}, nil
}
