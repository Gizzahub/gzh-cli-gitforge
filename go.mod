module github.com/gizzahub/gzh-cli-gitforge

go 1.25.1

require (
	github.com/fsnotify/fsnotify v1.9.0
	github.com/gizzahub/gzh-cli-core v0.0.0-20251230045225-725b628c716a
	github.com/google/go-github/v66 v66.0.0
	github.com/spf13/cobra v1.10.2
	github.com/xanzy/go-gitlab v0.115.0
	golang.org/x/oauth2 v0.34.0
	golang.org/x/sync v0.19.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/gizzahub/gzh-cli-core => ../gzh-cli-core

require (
	github.com/fatih/color v1.18.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/time v0.14.0 // indirect
)
