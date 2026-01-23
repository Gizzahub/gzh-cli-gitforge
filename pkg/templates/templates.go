// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package templates provides embedded YAML templates for configuration generation.
// Templates are compiled into the binary using Go's embed package.
package templates

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed *.yaml
var fs embed.FS

// TemplateName identifies available templates.
type TemplateName string

const (
	// RepositoriesBasic is a basic repositories config with flat repo list.
	RepositoriesBasic TemplateName = "repositories-basic.yaml"

	// RepositoriesChild is a child config that inherits from parent.
	RepositoriesChild TemplateName = "repositories-child.yaml"

	// RepositoriesBootstrap is a minimal bootstrap config for auto-generation.
	RepositoriesBootstrap TemplateName = "repositories-bootstrap.yaml"

	// WorkspaceWorkstation is a top-level workstation config with profiles.
	WorkspaceWorkstation TemplateName = "workspace-workstation.yaml"

	// WorkspaceForge is a workspace config with forge source integration.
	WorkspaceForge TemplateName = "workspace-forge.yaml"

	// Profile is a standalone profile configuration.
	Profile TemplateName = "profile.yaml"
)

// GetRaw returns the raw template content without processing.
func GetRaw(name TemplateName) ([]byte, error) {
	return fs.ReadFile(string(name))
}

// Render renders a template with the given data.
func Render(name TemplateName, data any) (string, error) {
	content, err := fs.ReadFile(string(name))
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", name, err)
	}

	tmpl, err := template.New(string(name)).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}

	return buf.String(), nil
}

// MustRender renders a template and panics on error.
// Use only when template errors are programming bugs.
func MustRender(name TemplateName, data any) string {
	result, err := Render(name, data)
	if err != nil {
		panic(err)
	}
	return result
}

// List returns all available template names.
func List() []TemplateName {
	return []TemplateName{
		RepositoriesBasic,
		RepositoriesChild,
		RepositoriesBootstrap,
		WorkspaceWorkstation,
		WorkspaceForge,
		Profile,
	}
}

// ================================================================================
// Template Data Structures
// ================================================================================

// RepositoriesBasicData is the data for RepositoriesBasic template.
type RepositoriesBasicData struct {
	Name         string
	Team         string
	Description  string
	Strategy     string // reset|pull|fetch
	Parallel     int
	Repositories []RepoData
}

// RepoData represents a single repository entry.
type RepoData struct {
	Name        string
	URL         string
	Path        string
	Branch      string
	Description string
}

// RepositoriesChildData is the data for RepositoriesChild template.
type RepositoriesChildData struct {
	Parent       string
	Profile      string
	Repositories []RepoData
}

// BootstrapData is the data for RepositoriesBootstrap template.
type BootstrapData struct {
	Parent  string
	Profile string
}

// WorkstationData is the data for WorkspaceWorkstation template.
type WorkstationData struct {
	Name       string
	Owner      string
	Parallel   int
	CloneProto string
	Profiles   map[string]ProfileData
	Workspaces map[string]WorkspaceData
}

// ProfileData represents a profile entry.
type ProfileData struct {
	Provider         string
	BaseURL          string
	Token            string
	CloneProto       string
	SSHPort          int
	Parallel         int
	IncludeSubgroups bool
	SubgroupMode     string
	Sync             *SyncData
}

// SyncData represents sync settings.
type SyncData struct {
	Strategy   string
	MaxRetries int
}

// WorkspaceData represents a workspace entry.
type WorkspaceData struct {
	Path     string
	Type     string // forge|git|config
	Profile  string
	Source   *ForgeSourceData
	Sync     *SyncData
	Parallel int
}

// ForgeSourceData represents a forge source.
type ForgeSourceData struct {
	Provider         string
	Org              string
	BaseURL          string
	Token            string
	IncludeSubgroups bool
	SubgroupMode     string
}

// WorkspaceForgeData is the data for WorkspaceForge template.
type WorkspaceForgeData struct {
	Parent     string
	Profile    string
	Name       string
	Team       string
	Workspaces map[string]WorkspaceData
}
