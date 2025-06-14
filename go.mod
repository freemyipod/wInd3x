module github.com/freemyipod/wInd3x

go 1.23.0

toolchain go1.23.3

require (
	github.com/adrg/xdg v0.4.0
	github.com/google/gousb v1.1.1
	github.com/hashicorp/go-multierror v1.1.1
	github.com/spf13/cobra v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/tetratelabs/wazero v1.0.0-pre.1
	github.com/ulikunitz/xz v0.5.12
	golang.org/x/exp v0.0.0-20250305212735-054e65f0b394
	howett.net/plist v1.0.0
)

require (
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	golang.org/x/sys v0.0.0-20220412211240-33da011f77ad // indirect
)

// Necessary for https://github.com/DHowett/go-plist/pull/76
replace howett.net/plist v1.0.0 => github.com/q3k/go-plist v0.0.0-20230225213725-6b5035fad602
