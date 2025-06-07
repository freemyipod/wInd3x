import {html, css, LitElement, PropertyValues} from 'lit';
import {Task, TaskStatus} from '@lit/task';
import {customElement, property, state} from 'lit/decorators.js';

import { load as loadGo, Exports, PayloadKind, InterfaceKind, DeviceDescription, App as GoApp, DeviceKind } from "./go.js";
import { load as loadEdk2} from "./edk2.js";

var go: Exports | null = null;

const commonStyles = css`
    .attention {
        background-color: #ffeeee;
        padding: 0.5em 2em 0.5em 2em;
    }
    .okay {
        background-color: #eeffee;
        padding: 0.5em 2em 0.5em 2em;
    }
    button {
        padding: 0.5em;
    }
    h1 small {
        color: #444;
    }
    li {
        line-height: 1.5em;
    }
    p {
        line-height: 1.5em;
    }
`;

@customElement("nz-disclaimer")
export class Disclaimer extends LitElement {
    static styles = [commonStyles];

    @state()
    protected _accepted: boolean = false;

    @state()
    protected _toast: string = "";

    @state()
    _checked: boolean = false;

    render() {
        return html`
            <h1>Welcome to the nugget zone! <small>alpha 2</small></h1>
            <p>
                This little web tool is a proof of concept to demonstrate custom-firmware-like capabilities on the <b>iPod Nano 7th Gen</b>. It allows you to run a customized version of the stock software <b>fully in memory and reversible by reboot</b>.
            </p>
            <p>
                Current customizations are:
            </p>
            <p>
                <ol>
                    <li>'freemyipod' branding on the USB connection screen.</li>
                </ol>
            </p>
            <p>
                This tool is based upon the <a href="https://github.com/freemyipod/wInd3x">wInd3x</a> toolkit</a>, but runs fully in your browser. It makes use of multiple vulnerabilities and exploit chains discovered by many people, eg.: __gsch, q3k, and others.
            </p>
            <p>
                It maintained by <a href="https://social.hackerspace.pl/@q3k">q3k</a>, who can be reached at q3k@q3k.org by email or @q3k:hackerspace.pl on Matrix.
            </p>
            <div class="attention">
                <h4>Warranty Disclaimer</h4>
                <p>
                    Nugget.zone is <b>experimental software</b> with <b>absolutely no warranty given</b>. While iPods are quite resiliant to bricking, they are not fully immune to it. No bricking has been reported as result of using this software, but that doesn't mean it can't happen - you have been warned.
                </p>
                <p>
                    THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
                </p>
            </div>
            <div class="actions">
                <p>
                    <input
                        id="accept-tos" type="checkbox"
                        .checked="${this._accepted}" 
                        @change="${(e: {target: HTMLInputElement}) => this._accepted = e.target.checked}"
                    />
                    <label for="accept-tos">I understand and accept the disclaimer above and wish to continue.</label>
                </p>
                ${this._toast != "" ? html`<p class="toast attention">${this._toast}</p>` : html``}
                <button @click=${this.continue}>Continue</button>
            </div>
        `;
    }

    continue() {
        if (this._accepted) {
            if (navigator.usb === undefined) {
                alert("Your browser doesn't support WebUSB. Please use Chrome/Chromium/Edge instead.")
            } else {
                this.dispatchEvent(new CustomEvent('accepted', {
                    detail: {
                        usb: navigator.usb,
                    },
                }));
            }
        } else {
            this._toast = "Please read and accept the disclaimer above before continuing.";
        }
    }
}

class RunError extends Error {}

@customElement("nz-device-connected")
export class DeviceConnected extends LitElement {
    render() {
    }
}

interface Connected {
    app: GoApp,
    di: DeviceDescription,
    manufacturer?: string,
}

@customElement("nz-hex-dump")
export class HexDump extends LitElement {
    static styles = [commonStyles, css`
        pre {
            height: 32em;
            overflow-y: scroll;
            background-color: #280040;
            color: #ffffff;
            padding: 1em;
        }
    `];

    private _dump(array: Array<number>, offset: bigint): string {
        let lines = [];
        for (let i = 0; i < array.length; i += 16) {
            const block = array.slice(i, Math.min(i + 16, array.length));
            const addr = BigInt(offset) + BigInt(i);
            const addrStr = ("00000000" + addr.toString(16)).slice(-8);
            const hex = block.map((n) => {
                const v = ("0" + n.toString(16)).slice(-2);
                return v;
            })
            const ascii = block.map((n) => {
                let v = ".";
                if (n >= 0x20 && n <= 0x7e) {
                    v = String.fromCharCode(n);
                }
                return v;
            })
            lines.push(addrStr + "| " + hex.join(" ") + " |" + ascii.join(""));
        }
        return lines.join("\n");
    }

    @property()
    data: Array<number> = []

    @property()
    offset: bigint = BigInt(0)

    render() {
        return html`
            <pre>${this._dump(this.data, this.offset)}</pre>
        `;
    }
}

interface Step {
    description: string,
    percentage: number,
}

class Progress {
    steps: Array<Step>;

    constructor() {
        this.steps = [];
    }

    reset() {
        this.steps = [];
    }

    step(description: string): Step {
        console.log("Progress: Starting step " + description);
        const prev = this.steps.length;
        if (prev > 0) {
            this.steps[prev-1].percentage = 1.0;
        }
        const step = {
            description,
            percentage: 0,
        };
        this.steps.push(step);
        return step;
    }
}

@customElement("nz-wtf-device")
export class WTFDevice extends LitElement {
    @property()
    connected?: Connected

    @state()
    _progress: Progress = new Progress;

    @state()
    _bootrom: Array<number> = [];

    static styles = [commonStyles];

    private _cfwTask = new Task(this, {
        task: async ([], {signal}) => {
            if (this.connected === undefined) {
                throw new RunError("not connected");
            }
            this._progress.reset();

            let step = this._progress.step("Uploading RetailOS CFW...");
            await this.connected.app.SendPayload(PayloadKind.RetailOSCustomized, (done) => {
                step.percentage = done;
                this.requestUpdate();
            });
            await this.requestUpdate();

            this.dispatchEvent(new CustomEvent('started-cfw', {
                detail: {},
            }));
        },
        args: () => [],
        autoRun: false,
    });

    render() {
        let cfw = this._cfwTask.render({
            initial: () => html`
                <p>
                    <button @click=${this.sendCFW}>Upload CFW RetailOS</button>
                </p>
            `,
            pending: () => html``,
            complete: () => html`
                <p>
                    RetailOS CFW uploaded! To return to stock, simply reboot your device.
                </p>
            `,
            error: (e) => html`
                <p class="attention">
                    RetailOS CFW upload failed: <code>${e}</code>.
                </p>
                <p>
                    <button @click=${this.sendCFW}>Try again...</button>
                </p>
            `,
        });
        let progress = this._progress.steps.map((step) => {
            return html`
                <li>${step.description}: ${Math.floor(step.percentage * 100)}%</li>
            `;
        });
        let progress_ = html`
            <p>
                <ul class="okay">
                    ${progress}
                </ul>
            </p>
        `;
        return html`
            <p>
                You can now upload a customized RetailOS firmware.
            </p>
            ${progress.length > 0 ? progress_ : html``}
            ${cfw}
        `;
    }

    sendCFW() {
        this._bootrom = [];
        this._cfwTask.run();
    }
}

@customElement("nz-dfu-device")
export class DFUDevice extends LitElement {
    @property()
    connected?: Connected

    @state()
    _progress: Progress = new Progress;

    @state()
    _bootrom: Array<number> = [];

    @state()
    _disabled = false;

    static styles = [commonStyles];

    private _dumpTask = new Task(this, {
        task: async ([], {signal}) => {
            if (this.connected === undefined) {
                throw new RunError("not connected");
            }
            this._progress.reset();

            this._progress.step("Running S5Late...");
            await this.connected.app.PrepareUSB();

            let step = this._progress.step("Dumping BootROM...");

            const size = 0x10000;
            const blockSize = 0x40;
            const blocks = size / blockSize;
            for (let i = 0; i < blocks; i++) {
                let mem = await this.connected.app.DumpMem(BigInt("0x20000000") + BigInt(i) * BigInt(blockSize));
                let memArray = Array.from(mem);
                this._bootrom.push(...memArray);
                step.percentage = i / blocks;
                this.requestUpdate();
            }
            step.percentage = 1;
        },
        args: () => [],
        autoRun: false,
    });

    private _saveTask = new Task(this, {
        task: async ([], {signal}) => {
            if (this._bootrom.length === 0) {
                throw new RunError("No BootROM dump!")
            }
            const handle = await showSaveFilePicker({
                startIn: "downloads",
                suggestedName: "bootrom-n7g.bin",
                types: [{description: "Raw Binary", accept: {"application/octet-stream": ".bin"}}]
            });
            const writable = await handle.createWritable();
            const array = new Uint8Array(this._bootrom);
            await writable.write(array);
            await writable.close();
        },
        args: () => [],
        autoRun: false,
    });

    private async prepareStep(kind: PayloadKind, name: string) {
            if (this.connected === undefined) {
                throw new RunError("not connected");
            }

            let step = this._progress.step(name);
            await this.requestUpdate();
            await this.connected.app.PreparePayload(kind, (done) => {
                step.percentage = done;
                this.requestUpdate();
            });
            step.percentage = 1;
            await this.requestUpdate();
    }

    private _wtfTask = new Task(this, {
        task: async ([], {signal}) => {
            if (this.connected === undefined) {
                throw new RunError("not connected");
            }
            this._progress.reset();

            await this.prepareStep(PayloadKind.WTFUpstream, "Downloading WTF...");
            await this.prepareStep(PayloadKind.WTFDecrypted, "Decrypting WTF...");
            await this.prepareStep(PayloadKind.WTFDefanged, "Defanging WTF...");
            await this.prepareStep(PayloadKind.RetailOSUpstream, "Downloading RetailOS...");
            await this.prepareStep(PayloadKind.RetailOSDecrypted, "Decrypting RetailOS (this will take a while)...");

            let step = this._progress.step("Triggering HaxedDFU mode...");
            await this.connected.app.HaxDFU();
            step.percentage = 1;
            await this.requestUpdate();

            step = this._progress.step("Uploading defanged WTF...");
            await this.connected.app.SendPayload(PayloadKind.WTFDefanged, (done) => {
                step.percentage = done;
                this.requestUpdate();
            });
            await this.requestUpdate();

            this._disabled = true;
            this.dispatchEvent(new CustomEvent('switched-to-wtf', {
                detail: {},
            }));
        },
        args: () => [],
        autoRun: false,
    });

    render() {
        let nonePending = (this._dumpTask.status !== TaskStatus.PENDING)
            && (this._wtfTask.status !== TaskStatus.PENDING);

        let save = this._saveTask.render({
            initial: () => html`
                <button @click=${this.saveBootROM}>Save to file...</button>
            `,
            pending: () => html`
                Saving...
            `,
            complete: () => html`
                <button @click=${this.saveBootROM}>Save to file...</button>
                Saved!
            `,
            error: (e) => html`
                <button @click=${this.saveBootROM}>Save to file...</button>
                Error: <code>${e}</code>
            `,
        });
        let dump = this._dumpTask.render({
            complete: () => html`
                <p>
                    BootROM dump complete! ${save}
                </p>
                <nz-hex-dump .data=${this._bootrom} .offset=${BigInt("0x20000000")} />
            `,
            error: (e) => html`
                <p class="attention">
                    BootROM dump failed: <code>${e}</code>.
                </p>
            `,
        });
        let wtf = this._wtfTask.render({
            complete: () => html`
                <p>
                    Defanged WTF uploaded!
                </p>
            `,
            error: (e) => html`
                <p class="attention">
                    Defanged WTF upload failed:: <code>${e}</code>.
                </p>
            `,
        });
        let button = (nonePending && !this._disabled) ? html`
            <p>
                <button @click=${this.dumpBootROM}>Dump BootROM...</button>
                <button @click=${this.sendDefangedWTF}>Send defanged WTF...</button>
            </p>
        ` : html``;
        let progress = this._progress.steps.map((step) => {
            return html`
                <li>${step.description}: ${Math.floor(step.percentage * 100)}%</li>
            `;
        });
        let progress_ = html`
            <p>
                <ul class="okay">
                    ${progress}
                </ul>
            </p>
        `;
        return html`
            <p>
                You can now select to either dump the device's BootROM (useful for developers) or continue booting a CFW by sending a Defanged WTF (a second stage bootloader with signature checks disabled).
            </p>
            ${progress.length > 0 ? progress_ : html``}
            ${wtf}
            ${dump}
            ${button}
        `;
    }

    saveBootROM() {
        this._saveTask.run();
    }

    dumpBootROM() {
        this._bootrom = [];
        this._dumpTask.run();
    }

    sendDefangedWTF() {
        this._bootrom = [];
        this._wtfTask.run();
    }
}

@customElement("nz-main")
export class Main extends LitElement {
    @property()
    usb: USB | null = null;

    static styles = [commonStyles];

    private _loadTask = new Task(this, {
        task: async ([], {signal}) => {
            if (go === null) {
                const edk2 = await loadEdk2();
                go = await loadGo();
                await go.setup(edk2);
            }
        },
        args: () => [],
    });

    private async _connect(): Promise<Connected> {
        if (this.usb === null) {
            throw new RunError("no WebUSB support - how did you get here?");
        }
        if (go === null) {
            throw new RunError("go/wasm not loaded");
        }
        let usb = this.usb;
        let device = await usb.requestDevice({
            filters: [
                // Request everything by Apple...
                {vendorId: 0x05ac},
            ]
        });
        let app = await go.newApp(device);
        const di = await app.GetDeviceDescription();

        let manufacturer = "unknown";
        if (di.interfaceKind != InterfaceKind.Disk) {
            await app.PrepareUSB();
            manufacturer = (await app.GetStringDescriptors()).manufacturer
        }
        return {app, di, manufacturer};
    }

    private _wtfTask = new Task(this, {
        task: async ([], {signal}): Promise<Connected> => {
            return await this._connect();
        },
        autoRun: false,
        args: () => [],
    });

    private _dfuTask = new Task(this, {
        task: async ([], {signal}): Promise<Connected> => {
            return await this._connect();
        },
        autoRun: false,
        args: () => [],
    });

    private runDFU() {
        this._dfuTask.run();
    }

    private runWTF() {
        this._wtfTask.run();
    }

    @state()
    private _inWTF: boolean = false;

    private switchedToWTF() {
        this._inWTF = true;
    }

    private startedCFW() {
    }


    @state()
    private _hideInstructions: boolean = false;

    render() {
        let dfu = this._dfuTask.render({
            initial: () => html`
                <p>
                    <button @click=${this.runDFU}>Run!</button>
                </p>
            `,
            pending: () => html`
                <p>
                    Waiting for device...
                </p>
            `,
            complete: (c: Connected) => {
                if (c.di.interfaceKind === InterfaceKind.DFU) {
                    if (c.di.kind === DeviceKind.Nano7) {
                        this._hideInstructions = true;
                        return html`
                            <p>
                                Connected to ${c.di.kind} over ${c.di.interfaceKind}. <b>All good!</b>
                            </p>
                            <nz-dfu-device
                                .connected=${c}
                                @switched-to-wtf=${this.switchedToWTF}
                            />
                        `;
                    } else {
                        return html`
                            <p class="attention">
                                Connected to ${c.di.kind} over ${c.di.interfaceKind}. <b>That's not an iPod Nano 7G!</b>
                            </p>
                            <p>
                                <button @click=${this.runDFU}>Try again!</button>
                            </p>
                        `;
                    }
                } else {
                    return html`
                        <p class="attention">
                            Connected to ${c.di.kind} over ${c.di.interfaceKind}. <b>That's not DFU mode!</b> Please follow the instructions above and try again.
                        </p>
                        <p>
                            <button @click=${this.runDFU}>Try again!</button>
                        </p>
                    `;
                }
            },
            error: (e) => html`
                <p class="attention">
                    Could not connect to device: <code>${e}</code>
                </p>
                <p>
                    <button @click=${this.runDFU}>Try again...</button>
                </p>
            `,
        });
        let wtf = this._wtfTask.render({
            initial: () => html`
                <p>
                    The device should have switched to defanged WTF mode now. You will need to reconnect to it:
                </p>
                <p>
                    <button @click=${this.runWTF}>Connect!</button>
                </p>
            `,
            pending: () => html`
                <p>
                    Waiting for device...
                </p>
            `,
            complete: (c: Connected) => {
                if (c.di.interfaceKind === InterfaceKind.WTF && c.manufacturer === "freemyipod") {
                    if (c.di.kind === DeviceKind.Nano7) {
                        this._hideInstructions = true;
                        return html`
                            <p>
                                Connected to ${c.di.kind} over ${c.di.interfaceKind} with manufacturer ${c.manufacturer}. <b>All good!</b>
                            </p>
                            <nz-wtf-device
                                .connected=${c}
                                @started-cfw=${this.startedCFW}
                            />
                        `;
                    } else {
                        return html`
                            <p class="attention">
                                Connected to ${c.di.kind} over ${c.di.interfaceKind} with manufacturer ${c.manufacturer}. <b>That's not an iPod Nano 7G! What are you even doing..?</b>
                            </p>
                            <p>
                                <button @click=${this.runWTF}>Try again in WTF mode...</button>
                                <button @click=${this.runDFU}>... or from scratch in DFU mode.</button>
                            </p>
                        `;
                    }
                } else {
                    return html`
                        <p class="attention">
                            Connected to ${c.di.kind} over ${c.di.interfaceKind} with manufacturer ${c.manufacturer}. <b>That's not defanged WTF mode!</b> Something went wrong. Try again from scratch after restarting your device into DFU mode.
                        </p>
                        <p>
                            <button @click=${this.runDFU}>Try again!</button>
                        </p>
                    `;
                }
            },
            error: (e) => html`
                <p class="attention">
                    Could not connect to device: <code>${e}</code>
                </p>
                <p>
                    <button @click=${this.runWTF}>Try again in WTF mode...</button>
                    <button @click=${this.runDFU}>... or from scratch in DFU mode.</button>
                </p>
            `,
        });
        return this._loadTask.render({
            initial: () => html`
                <p>dingus</p>
            `,
            pending: () => html`
                <h1>Loading wInd3x and edk2...</h1>
                <p>
                    This might take a while (~10MiB to download!).
                </p>
            `,
            error: (e) => html`
                <h1>Loading wInd3x and edk2...</h1>
                <p>
                    Load error: ${e}
                </p>
            `,
            complete: () => {
                let instructions = html`
                    <p>
                        <b>Please read these instructions carefully!</b>
                    </p>
                    <ol>
                        <li>Connect your iPod Nano 7G to this computer. <b>No other generation is currently supported!</b></li>
                        <li>Press the button below - but read the rest of the instructions first!</li>
                        <li>A list of compatible devices will appear. It should contain an Apple iPod in disk/retail mode (“<code>iPod</code>”).</li>
                        <li>Switch the iPod into DFU mode by holding the home and power buttons. The iPod will reboot once (showing the Apple logo), then again (showing a black screen). Release the buttons when you see a device in DFU mode (“<code>USB DFU Device</code>”) appear on the list.</li>
                        <li>Select the DFU device from the list and allow access to it.</li>
                    </ol>
                    <p>
                        <i>Note:</i> Some USB-C to Lightning cables have been observed to not work with DFU mode, with the device not showing up over USB and immediately rebooting. If you're having issues, try a USB A to Lightning cable.
                    </p>
                `;
                return html`
                    <h1>Let's go!</h1>
                    ${this._hideInstructions ? html`` : instructions}
                    ${dfu}
                    ${this._inWTF ? wtf : html``}
                `;
            },
        });
    }
}

@customElement("nz-root")
export class Root extends LitElement {
    static styles = [commonStyles, css`
        div.root {
            max-width: 40em;
            margin-left: auto;
            margin-right: auto;
        }
        div.footer {
            margin-top: 2em;
            font-size: 90%;
            font-style: italic;
            background-color: #f8f8f8;
            padding: 0.5em 2em 0.5em 2em;
        }
    `];

    @state()
    private _show_disclaimer: boolean = true;

    private _usb: USB | null = null;

    render() {
        let inner = this._show_disclaimer
            ? html`<nz-disclaimer @accepted="${(e: CustomEvent) => { this._usb = e.detail.usb; this._show_disclaimer = false; }}" />`
            : html`<nz-main .usb=${this._usb} />`;
        return html`
            <div class="root">
                ${inner}
                <div class="footer">
                    nugget.zone and wInd3x are Free Software. <a href="https://github.com/freemyipod/wInd3x">Source code.</a>
                </div>
            </div>
        `;
    }
}