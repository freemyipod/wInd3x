import { Compression } from "./edk2.js";

// Kep in sync with pkg/cache/cache.go PayloadKind
export enum PayloadKind {
	WTFUpstream       = "wtf-upstream",
	WTFDecrypted      = "wtf-decrypted",
	WTFDecryptedCache = "wtf-decrypted-cache",
	WTFDefanged       = "wtf-defanged",

	RecoveryUpstream = "recovery-upstream",

	FirmwareUpstream = "firmware-upstream",

	BootloaderUpstream       = "bootloader-upstream",
	BootloaderDecrypted      = "bootloader-decrypted",
	BootloaderDecryptedCache = "bootloader-decrypted-cache",

	RetailOSUpstream = "retailos-upstream",

	DiagsUpstream       = "diags-upstream",
	DiagsDecrypted      = "diags-decrypted",
	DiagsDecryptedCache = "diags-decrypted-cache",

	JingleXML = "jinglexml",
}

// Keep in sync with pkgs/devices/devices.go Kind
export enum DeviceKind {
    Nano3 = "n3g",
    Nano4 = "n4g",
    Nano5 = "n5g",
    Nano6 = "n6g",
    Nano7 = "n7g",
}

// keep in sync with pkg/devices/devices.go InterfaceKind
export enum InterfaceKind {
    DFU = "dfu",
    WTF = "wtf",
    Disk = "diskmode",
}

export interface DeviceDescription {
    vid: number,
    pid: number,
    updaterFamilyID: number,
    kind: DeviceKind,
    interfaceKind: InterfaceKind,
}

interface StringDescriptors {
    manufacturer: string,
    product: string,
}

export interface App {
    GetStringDescriptors(): Promise<StringDescriptors>;
    GetDeviceDescription(): Promise<DeviceDescription>;
    PrepareUSB(): Promise<null>;
    HaxDFU(): Promise<null>;
    DumpMem(offset: BigInt): Promise<Uint8Array>;
    SendPayload(kind: PayloadKind): Promise<null>;
}

export interface Exports {
    newApp(usb: USBDevice): Promise<App>;
    setup(compression: Compression): Promise<null>;
}

declare class Go {
    argv: string[];
    env: { [envKey: string]: string };
    exit: (code: number) => void;
    importObject: WebAssembly.Imports;
    exited: boolean;
    mem: DataView;
    run(instance: WebAssembly.Instance): Promise<void>;
}

const go = new Go();

export async function load(): Promise<Exports> {
    const obj = await WebAssembly.instantiateStreaming(fetch("wiwali.wasm"), go.importObject);
    const wasm = obj.instance;
    go.run(wasm);

    const exports = (window as any as {wiwali: Exports}).wiwali;
    return exports;
}