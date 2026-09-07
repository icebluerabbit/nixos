module homelab/clients

go 1.26

require (
	github.com/google/go-cmp v0.7.0
	homelab v0.0.0
)

require (
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
)

replace homelab => ../lib
