#! /usr/bin/env nix-shell
#! nix-shell -i bash -p emscripten
set -e -x -o pipefail

# Rebuild edk2.wasm from EDK2 sources. It contains TianoCompress/Decompress
# functions which are then made available to the Go runtime using wazero.

[ -d edk2 ] || exit 1

# Work around Nix badness.
# See: https://github.com/NixOS/nixpkgs/issues/139943

emscriptenpath="$(dirname $(dirname $(which emcc)))"
if [ ! -d ~/.emscripten_cache ]; then
    cp -rv "$emscriptenpath/share/emscripten/cache" ~/.emscripten_cache
    chmod u+rwX -R ~/.emscripten_cache
fi
export EM_CACHE=~/.emscripten_cache

emcc \
    edk2/BaseTools/Source/C/Common/TianoCompress.c \
    edk2/BaseTools/Source/C/Common/Decompress.c \
    -I edk2/BaseTools/Source/C/Include/ \
    -I edk2/BaseTools/Source/C/Include/X64/ \
    -s EXPORTED_FUNCTIONS=_TianoDecompress,_TianoCompress,_malloc,_free \
    -s ALLOW_MEMORY_GROWTH \
    --no-entry \
    -O3 \
    -o edk2.wasm
