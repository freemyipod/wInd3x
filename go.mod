module github.com/freemyipod/wInd3x

go 1.19

require (
	github.com/adrg/xdg v0.4.0
	github.com/golang/glog v1.0.0
	github.com/google/gousb v1.1.1
	github.com/hashicorp/go-multierror v1.1.1
	github.com/spf13/cobra v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/tetratelabs/wazero v1.0.0-rc.2
	howett.net/plist v1.0.0
)

require (
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	golang.org/x/sys v0.0.0-20211025201205-69cdffdb9359 // indirect
)

// Necessary for https://github.com/DHowett/go-plist/pull/76
replace howett.net/plist v1.0.0 => github.com/q3k/go-plist v0.0.0-20230225213725-6b5035fad602
