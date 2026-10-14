// Package forgebootstrap plans and applies independent repository initialization
// from an exact template commit or a digest-bound initial template seed.
//
// Plans bind site configuration, safe checkout paths and observed GitLab project
// identity. Authentication remains outside persisted plans and provenance. Apply
// records recoverable phases, retains resources on failure and verifies an atomic
// publication of the master and develop branches against the initial commit.
// The authenticated CLI boundary excludes local file remotes; tests can provide
// a fake Remote and local Git fixtures without sending credentials to a forge.
package forgebootstrap
