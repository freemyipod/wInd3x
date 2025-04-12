export interface Compression {
    compress(data: Uint8Array): Uint8Array;
    decompress(data: Uint8Array): Uint8Array;
}

interface Exports {
    malloc(size: number): number;
    free(size: number): number;
    TianoCompress(in_ptr: number, in_size: number, out_ptr: number, out_size: number): bigint;
    TianoDecompress(in_ptr: number, in_size: number, out_ptr: number, out_size: number, scratch_ptr: number, scratch_size: number): bigint;
    memory: WebAssembly.Memory;
}

export async function load(): Promise<Compression> {
    const moduleMemory = new WebAssembly.Memory({initial: 512, maximum: 4096, shared: true})
    const importedObject = {
        env: {
            __memory_base: 0,
            memory: moduleMemory,
            emscripten_notify_memory_growth: function(index: any) {
                // Do we need to do anything here?
                console.log("emscripten_notify_memory_growth", index);
            },
        },
    }
    const obj = await WebAssembly.instantiateStreaming(fetch("edk2.wasm"), importedObject);
    const wasm = obj.instance;
    const { malloc, free, TianoCompress, TianoDecompress, memory } = wasm.exports as any as Exports;
    return {
        compress: (input: Uint8Array) => {
            if (input.length == 0) {
                throw Error("cannot compress empty file");
            }
            const in_ptr = malloc(input.length);
            const in_buf = new Uint8Array(memory.buffer, in_ptr, input.length);
            in_buf.set(input);

            const out_ptr = malloc(input.length);
            const out_size_ptr = malloc(4);
            let out_size_buf = new Uint32Array(memory.buffer, out_size_ptr, 1);
            out_size_buf[0] = input.length;

            let res = TianoCompress(in_ptr, input.length, out_ptr, out_size_ptr);
            if (res != BigInt(0)) {
                throw new Error("TianoCompress returned non-zero result" + res.toString());
            }

            out_size_buf = new Uint32Array(memory.buffer, out_size_ptr, 1);
            let out_buf = new Uint8Array(memory.buffer, out_ptr, out_size_buf[0]);

            free(in_ptr);
            free(out_ptr);
            return out_buf;
        },
        decompress: (input: Uint8Array) => {
            const in_ptr = malloc(input.length);
            const in_buf = new Uint8Array(memory.buffer, in_ptr, input.length);
            in_buf.set(input);

            const out_len = (new Uint32Array(input))[4];
            const out_ptr = malloc(out_len);

            const scratch_len = 13393;
            const scratch_ptr = malloc(scratch_len);

            let res = TianoDecompress(in_ptr, input.length, out_ptr, out_len, scratch_ptr, scratch_len);
            if (res != BigInt(0)) {
                throw new Error("TianoDecompress returned non-zero result " + res.toString());
            }

            let out_buf = new Uint8Array(memory.buffer, out_ptr, out_len);
            free(in_ptr);
            free(out_ptr);
            free(scratch_ptr);
            return out_buf
        },
    }
}