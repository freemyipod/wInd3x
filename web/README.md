nugget.zone
===========

Alpha-stage web interface to wInd3x.

Architecture
------------

wInd3x is exported as `wiwali` (the wInd3x Wasm Library), which is compiled to WebAssembly and loaded by the Typescript/ECMAScript app.

Because wInd3x itself already uses WebAssembly (via wazero) to include the TianoCore compression algorithm, it has to be injected by the app at startup to provide it with the same functionality from the same Emscripten bundle, but not loaded by the browser instead of wazero.

The Typescript/ECMAScript app is frameworkless and based around lit.js WebComponents.

Getting started
---------------

    $ nix-shell
    $ make run

To just package, do `make dist` and deploy `dist` to your webserver.

License
-------

Unlike the rest of wInd3x, the nugget.zone codebase (Go bindings, Typescript code, etc.) is AGPL licensed. See COPYING for more details.
