// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateConfigSymlink(t *testing.T) {
	// Create temp directories for testing
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "configs")
	targetDir := filepath.Join(tmpDir, "workspace")

	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}

	// Create source config file
	srcPath := filepath.Join(srcDir, "myconfig.yaml")
	if err := os.WriteFile(srcPath, []byte("test: true"), 0o644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	tests := []struct {
		name            string
		srcPath         string
		targetDir       string
		parentConfigDir string
		setup           func() error
		wantErr         bool
		checkSymlink    bool
	}{
		{
			name:            "empty srcPath does nothing",
			srcPath:         "",
			targetDir:       targetDir,
			parentConfigDir: srcDir,
			wantErr:         false,
			checkSymlink:    false,
		},
		{
			name:            "absolute path",
			srcPath:         srcPath,
			targetDir:       filepath.Join(tmpDir, "ws1"),
			parentConfigDir: srcDir,
			wantErr:         false,
			checkSymlink:    true,
		},
		{
			name:            "relative path",
			srcPath:         "./myconfig.yaml",
			targetDir:       filepath.Join(tmpDir, "ws2"),
			parentConfigDir: srcDir,
			wantErr:         false,
			checkSymlink:    true,
		},
		{
			name:            "source does not exist",
			srcPath:         "/nonexistent/config.yaml",
			targetDir:       filepath.Join(tmpDir, "ws3"),
			parentConfigDir: srcDir,
			wantErr:         true,
			checkSymlink:    false,
		},
		{
			name:            "target dir created automatically",
			srcPath:         srcPath,
			targetDir:       filepath.Join(tmpDir, "newws", "deep", "path"),
			parentConfigDir: srcDir,
			wantErr:         false,
			checkSymlink:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				if err := tt.setup(); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}

			err := CreateConfigSymlink(tt.srcPath, tt.targetDir, tt.parentConfigDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateConfigSymlink() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkSymlink && err == nil {
				linkPath := filepath.Join(tt.targetDir, DefaultConfigFileName)
				isSymlink, err := IsConfigSymlink(linkPath)
				if err != nil {
					t.Errorf("IsConfigSymlink() error = %v", err)
					return
				}
				if !isSymlink {
					t.Error("Expected symlink to be created")
				}
			}
		})
	}
}

func TestCreateConfigSymlink_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source config
	srcPath := filepath.Join(tmpDir, "source.yaml")
	if err := os.WriteFile(srcPath, []byte("source: true"), 0o644); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	// Create target directory with existing regular file
	targetDir := filepath.Join(tmpDir, "workspace")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("Failed to create target dir: %v", err)
	}

	existingConfig := filepath.Join(targetDir, DefaultConfigFileName)
	if err := os.WriteFile(existingConfig, []byte("existing: true"), 0o644); err != nil {
		t.Fatalf("Failed to create existing config: %v", err)
	}

	// Should fail because regular file exists
	err := CreateConfigSymlink(srcPath, targetDir, tmpDir)
	if err == nil {
		t.Error("Expected error when regular file exists, got nil")
	}
}

func TestCreateConfigSymlink_ExistingSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two source configs
	src1 := filepath.Join(tmpDir, "config1.yaml")
	src2 := filepath.Join(tmpDir, "config2.yaml")
	if err := os.WriteFile(src1, []byte("config: 1"), 0o644); err != nil {
		t.Fatalf("Failed to create source1: %v", err)
	}
	if err := os.WriteFile(src2, []byte("config: 2"), 0o644); err != nil {
		t.Fatalf("Failed to create source2: %v", err)
	}

	// Create target directory with existing symlink
	targetDir := filepath.Join(tmpDir, "workspace")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("Failed to create target dir: %v", err)
	}

	linkPath := filepath.Join(targetDir, DefaultConfigFileName)
	if err := os.Symlink(src1, linkPath); err != nil {
		t.Fatalf("Failed to create initial symlink: %v", err)
	}

	// Should succeed and replace the symlink
	err := CreateConfigSymlink(src2, targetDir, tmpDir)
	if err != nil {
		t.Errorf("CreateConfigSymlink() error = %v, expected nil for existing symlink", err)
		return
	}

	// Verify symlink points to new source
	target, err := GetSymlinkTarget(linkPath)
	if err != nil {
		t.Errorf("GetSymlinkTarget() error = %v", err)
		return
	}
	if target != src2 {
		t.Errorf("Symlink target = %v, want %v", target, src2)
	}
}

func TestCreateConfigSymlinkForce(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source config
	srcPath := filepath.Join(tmpDir, "source.yaml")
	if err := os.WriteFile(srcPath, []byte("source: true"), 0o644); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	// Create target directory with existing regular file
	targetDir := filepath.Join(tmpDir, "workspace")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("Failed to create target dir: %v", err)
	}

	existingConfig := filepath.Join(targetDir, DefaultConfigFileName)
	if err := os.WriteFile(existingConfig, []byte("existing: true"), 0o644); err != nil {
		t.Fatalf("Failed to create existing config: %v", err)
	}

	// Force should succeed even with regular file
	err := CreateConfigSymlinkForce(srcPath, targetDir, tmpDir)
	if err != nil {
		t.Errorf("CreateConfigSymlinkForce() error = %v, expected nil", err)
		return
	}

	// Verify symlink was created
	isSymlink, err := IsConfigSymlink(existingConfig)
	if err != nil {
		t.Errorf("IsConfigSymlink() error = %v", err)
		return
	}
	if !isSymlink {
		t.Error("Expected symlink to be created")
	}
}

func TestResolveSymlinkPath(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		path     string // childPath (the path to resolve)
		baseDir  string // parentPath (the base directory)
		wantErr  bool
		validate func(result string) bool
	}{
		{
			name:    "absolute path unchanged",
			path:    "/absolute/path/file.yaml",
			baseDir: tmpDir,
			wantErr: false,
			validate: func(result string) bool {
				return result == "/absolute/path/file.yaml"
			},
		},
		{
			name:    "relative path joined with base",
			path:    "relative/file.yaml",
			baseDir: tmpDir,
			wantErr: false,
			validate: func(result string) bool {
				return result == filepath.Join(tmpDir, "relative/file.yaml")
			},
		},
		{
			name:    "dot-relative path",
			path:    "./file.yaml",
			baseDir: tmpDir,
			wantErr: false,
			validate: func(result string) bool {
				return result == filepath.Join(tmpDir, "./file.yaml")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// resolvePath(parentPath, childPath) - baseDir is parent, path is child
			result, err := resolvePath(tt.baseDir, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolvePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.validate(result) {
				t.Errorf("resolvePath() = %v, validation failed", result)
			}
		})
	}
}

func TestIsConfigSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular file
	regularFile := filepath.Join(tmpDir, "regular.yaml")
	if err := os.WriteFile(regularFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	// Create a symlink
	symlinkFile := filepath.Join(tmpDir, "symlink.yaml")
	if err := os.Symlink(regularFile, symlinkFile); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	tests := []struct {
		name       string
		path       string
		want       bool
		wantErr    bool
	}{
		{
			name:    "regular file",
			path:    regularFile,
			want:    false,
			wantErr: false,
		},
		{
			name:    "symlink",
			path:    symlinkFile,
			want:    true,
			wantErr: false,
		},
		{
			name:    "nonexistent",
			path:    filepath.Join(tmpDir, "nonexistent"),
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsConfigSymlink(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsConfigSymlink() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsConfigSymlink() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSymlinkTarget(t *testing.T) {
	tmpDir := t.TempDir()

	// Create target file
	targetFile := filepath.Join(tmpDir, "target.yaml")
	if err := os.WriteFile(targetFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	// Create symlink
	linkFile := filepath.Join(tmpDir, "link.yaml")
	if err := os.Symlink(targetFile, linkFile); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Test getting symlink target
	target, err := GetSymlinkTarget(linkFile)
	if err != nil {
		t.Errorf("GetSymlinkTarget() error = %v", err)
		return
	}
	if target != targetFile {
		t.Errorf("GetSymlinkTarget() = %v, want %v", target, targetFile)
	}

	// Test with non-symlink
	target, err = GetSymlinkTarget(targetFile)
	if err != nil {
		t.Errorf("GetSymlinkTarget() error = %v for regular file", err)
		return
	}
	if target != "" {
		t.Errorf("GetSymlinkTarget() = %v for regular file, want empty string", target)
	}
}
