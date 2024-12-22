module github.com/freemyipod/wInd3x

go 1.16

require (
	github.com/adrg/xdg v0.4.0
	github.com/golang/glog v1.0.0
	github.com/google/gousb v1.1.1
	github.com/hashicorp/go-multierror v1.1.1
	github.com/spf13/cobra v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/tetratelabs/wazero v1.0.0-pre.1
	github.com/ulikunitz/xz v0.5.12
	howett.net/plist v1.0.0
)

// Necessary for https://github.com/DHowett/go-plist/pull/76
replace howett.net/plist v1.0.0 => github.com/q3k/go-plist v0.0.0-20230225213725-6b5035fad602
