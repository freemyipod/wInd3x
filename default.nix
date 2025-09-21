{ system ? builtins.currentSystem, ... }:

let
  nixpkgsCommit = "8f3e1f807051e32d8c95cd12b9b421623850a34d";
  nixpkgsSrc = fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/${nixpkgsCommit}.tar.gz";
    sha256 = "sha256:1cpsxhm3fwsyf5shr35kirj1ajz8iy601q37xi3ma4f8dxd4vagy";
  };
  nixpkgs = import nixpkgsSrc { inherit system; };

in with nixpkgs;

buildGoModule {
  name = "wInd3x";
  src = ./.;

  buildInputs = [ libusb1 ];
  nativeBuildInputs = [ pkg-config ];

  checkPhase = "true";

  vendorHash = "sha256-Is7FCtoKmC/B2ktuysNHoxXUP+TPRs/0C2ZzdKgq0rE=";
}
