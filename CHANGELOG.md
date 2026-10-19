# Changelog

This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Release notes are generated, not written by hand. On each tag, `.goreleaser.yaml`
(`changelog.use: github`) publishes the per-commit list into the
[GitHub Releases](https://github.com/gizzahub/gzh-cli-gitforge/releases). The
reason behind a change belongs in its commit body, which the generated list
links to. Do not add `[Unreleased]` entries here, and do not hold a release for
one.

Announce a deprecation with a `Deprecated:` trailer in the commit message, next
to the other trailers, for example
`Deprecated: Client.Open; use Client.OpenContext, removal in v1.0.0`. The
release workflow collects these trailers into a "Deprecated" section of the
release notes (`scripts/release-deprecations.sh`). A `Deprecated:` line in the
middle of the body is not a trailer and is not collected. Keep each notice on
one line, or indent its continuation lines. An unindented continuation line
usually stops Git from reading the block as trailers, and the release only
warns about it; next to a `Signed-off-by:` trailer, Git keeps the block and the
notice is cut at its first line without a warning.

This file only points to the release history. The hand-written record ends with
v0.10.5; the table below links it.

______________________________________________________________________

## Hand-written history

v0.8.0 (2026-10-01) was the first stable published tag. Earlier 0.x files record
source-version milestones.

| Line                             | Milestones                                                        |
| -------------------------------- | ----------------------------------------------------------------- |
| [0.10.x](docs/changelog/0.10.md) | 0.10.5 (2026-10-19), 0.9.2–0.9.4 (2026-10-14)                     |
| [0.9.x](docs/changelog/0.9.md)   | 0.9.1 (2026-10-08), 0.9.0 (2026-10-02); 0.9.2–0.9.4 are in 0.10.x |
| [0.8.x](docs/changelog/0.8.md)   | 0.8.0 (2026-10-01)                                                |
| [0.7.x](docs/changelog/0.7.md)   | 0.7.0 (2026-07-02)                                                |
| [0.6.x](docs/changelog/0.6.md)   | 0.6.1 (2026-01-25), 0.6.0 (2026-01-21)                            |
| [0.4.x](docs/changelog/0.4.md)   | 0.4.0 (2026-01-02)                                                |
| [0.3.x](docs/changelog/0.3.md)   | 0.3.1 (2025-12-02), 0.3.0 (2025-12-01)                            |
| [0.2.x](docs/changelog/0.2.md)   | 0.2.0 (2025-12-01)                                                |
| [0.1.x](docs/changelog/0.1.md)   | 0.1.0-alpha (2025-12-01)                                          |

## Links

- **Repository**: https://github.com/gizzahub/gzh-cli-gitforge
- **Releases**: https://github.com/gizzahub/gzh-cli-gitforge/releases
- **Documentation**: https://pkg.go.dev/github.com/gizzahub/gzh-cli-gitforge
- **Issues**: https://github.com/gizzahub/gzh-cli-gitforge/issues
- **Discussions**: https://github.com/gizzahub/gzh-cli-gitforge/discussions

______________________________________________________________________

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) CLI framework
- Follows [Conventional Commits](https://www.conventionalcommits.org/) specification
- Inspired by [gzh-cli](https://github.com/gizzahub/gzh-cli)
