// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposync

// ProgressSink receives progress events from the executor.
type ProgressSink interface {
	OnStart(action Action)
	OnProgress(action Action, message string, progress float64)
	OnComplete(result ActionResult)
}

// NoopProgressSink is a progress sink that does nothing.
type NoopProgressSink struct{}

// OnStart implements ProgressSink.
func (NoopProgressSink) OnStart(_ Action) {}

// OnProgress implements ProgressSink.
func (NoopProgressSink) OnProgress(_ Action, _ string, _ float64) {}

// OnComplete implements ProgressSink.
func (NoopProgressSink) OnComplete(_ ActionResult) {}
