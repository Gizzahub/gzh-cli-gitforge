module github.com/gizzahub/gzh-cli-gitforge

go 1.24.0

require (
	github.com/google/go-github/v66 v66.0.0
	github.com/spf13/cobra v1.10.2
	github.com/xanzy/go-gitlab v0.115.0
	golang.org/x/oauth2 v0.24.0
	golang.org/x/sync v0.18.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/gizzahub/gzh-cli-core => ../gzh-cli-core

require (
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/time v0.3.0 // indirect
)

// For local development, uncomment the following line:
// replace github.com/gizzahub/gzh-cli-git => ../gzh-cli-git
