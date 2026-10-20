module example.com/collision-root

go 1.25.0

require (
	github.com/acme/a/common v0.0.0
	github.com/acme/b/common v0.0.0
)

replace github.com/acme/a/common => ../a/common

replace github.com/acme/b/common => ../b/common
