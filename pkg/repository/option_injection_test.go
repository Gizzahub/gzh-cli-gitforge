// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// optionInjectionRefs are external values that git would parse as options if
// passed as bare positional arguments (the --upload-pack=… class of attack).
var optionInjectionRefs = []string{
	"--upload-pack=/tmp/evil",
	"--output=/tmp/evil",
	"-x",
}

// TestClone_RejectsOptionInjection verifies that Clone rejects URLs and branches
// git could interpret as options, before any git process is spawned (AC4).
func TestClone_RejectsOptionInjection(t *testing.T) {
	ctx := context.Background()
	c := NewClient()
	dest := t.TempDir()

	t.Run("malicious URL", func(t *testing.T) {
		_, err := c.Clone(ctx, CloneOptions{
			URL:         "--upload-pack=/tmp/evil",
			Destination: dest,
		})
		var verr *ValidationError
		if !errors.As(err, &verr) || verr.Field != "URL" {
			t.Fatalf("expected URL ValidationError, got %v", err)
		}
	})

	t.Run("malicious branch", func(t *testing.T) {
		for _, br := range optionInjectionRefs {
			_, err := c.Clone(ctx, CloneOptions{
				URL:         "https://github.com/test/repo.git",
				Destination: dest,
				Branch:      br,
			})
			var verr *ValidationError
			if !errors.As(err, &verr) || verr.Field != "Branch" {
				t.Fatalf("branch %q: expected Branch ValidationError, got %v", br, err)
			}
		}
	})
}

// TestBulkSwitch_RejectsOptionInjection verifies BulkSwitch rejects a target
// branch git could read as an option, before scanning the directory (AC4).
func TestBulkSwitch_RejectsOptionInjection(t *testing.T) {
	ctx := context.Background()
	c := NewClient()
	for _, br := range optionInjectionRefs {
		_, err := c.BulkSwitch(ctx, BulkSwitchOptions{
			Directory: t.TempDir(),
			Branch:    br,
		})
		if err == nil || !strings.Contains(err.Error(), "invalid branch name") {
			t.Fatalf("branch %q: expected invalid branch name error, got %v", br, err)
		}
	}
}

// TestBulkTag_RejectsOptionInjection verifies BulkTag rejects a tag name git
// could read as an option, before scanning the directory (AC4).
func TestBulkTag_RejectsOptionInjection(t *testing.T) {
	ctx := context.Background()
	c := NewClient()
	for _, name := range optionInjectionRefs {
		_, err := c.BulkTag(ctx, BulkTagOptions{
			Directory: t.TempDir(),
			Operation: "create",
			TagName:   name,
		})
		if err == nil || !strings.Contains(err.Error(), "invalid tag name") {
			t.Fatalf("tag %q: expected invalid tag name error, got %v", name, err)
		}
	}
}
