// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"strings"
	"testing"
)

func TestInboundJSONDepthBound(t *testing.T) {
	// Four containers are required by the current plan's array fields.
	if err := checkInboundJSON([]byte(`{"a":{"b":{"c":["value"]}}}`)); err != nil {
		t.Fatalf("schema-depth JSON rejected: %v", err)
	}
	// The object is valid JSON with a valid field name, so this must fail
	// at the walker rather than later as an unknown typed field.
	deep := `{"x":` + strings.Repeat("[", 2048) + "0" + strings.Repeat("]", 2048) + "}"
	if err := checkInboundJSON([]byte(deep)); err == nil {
		t.Fatal("deeply nested JSON was accepted")
	}
}
