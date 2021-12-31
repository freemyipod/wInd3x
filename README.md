wind3x
======

Experimental Nano 4G (and maybe Nano 5G) bootrom/DFU exploit.

Building
--------

You'll need go, libusb, and keystone. Then:

    $ go build

Or, if you have Nix(OS), just do:

    $ nix-build

Running
-------

Put your iPod into DFU mode by connecting it over USB, holding down menu+select until it reboots, blanks the screen, then shows the Apple logo, then blanks the screen again. The iPod should enumerate as 'USB DFU Device'.

Then, run wind3x to put the iPod into 'haxed DFU' mode. This is a modified DFU mode that allows booting any DFU image, including unsigned and unencrypted ones. The mode is temporary, and will be active only until next (re)boot, the exploit does not modify the device permanently in any way.

    $ ./wind3x
    2021/12/31 00:59:13 wind3x - nano 4g bootrom exploit
    ...
    2021/12/31 00:59:15 Device will now accept signed and unsigned DFU images.

You can then use any DFU tool to upload any DFU image and the device should boot it. You can also pass the -image argument to wind3x to make it immediately send a file as a DFU image after running haxed DFU mode.

Haxed DFU Mode
--------------

Once in haxed DFU mode, the DFU will continue as previously, and you will still be able to send properly signed images (like WTF), or images which exploit Pwnage 2.0. In addition, you can now send over images with type sent to '0' instead of '3'/'4', which bypass all decryption and signature checks (header and footer), and which will be loaded and executed as is.

As an example, the following image will hang the iPod in a spinloop:

    # header declaring a 0x100 byte body and no footer
    payload = b'87202.0\x00\x00\x00\x00\x00\x00\x01\x00\x00'
    payload += b'\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00'
    # 'salt'
    payload += b'Z' * 32
    # unused security epoch
    payload += b'\x00\x00\x00\x00'
    # unchecked signature
    payload += b'Z' * 16
    # pad header to 0x600 bytes
    payload += b'\x00' * (0x600 - len(payload))
    # code goes here
    payload += b'\xfe\xff\xff\xea'
    # pad entire thing to 0x600 + 0x100 bytes
    payload += b'\x00' * (0x700 - len(payload))


Running EmCORE
--------------

To be documented.

Vulnerability
=============

This exploits a vulnerability in the standard SETUP packet parsing code of the bootrom, in which the wIndex parameter is not checked for bmRequest == {0x20, 0x40}, but is still used to index an array of interface/class handlers (that in the Bootrom has a length of 1).

Nano4G Exploit Chain
--------------------

We abuse the fact that wIndex == 3 for bmRequest 0x40 treats a 'bytes left to sent over USB' counter as a function pointer and calls it with r0 == address of SETUP. We massage the DFU mode into attempting to send us 0x3b0+0x40 bytes, and failing after 0x40 bytes, thereby leaving the counter at 0x3b0 bytes and executing code at address 0x3b0.

Since the bootrom is mapped at offset 0x0 as well as 0x20000000 at boot, this means we execute bootrom code, and 0x3b0 happens to point to a 'blx r0' instruction. This in turn causes the CPU to interpret the SETUP packet received as ARM code.

We specially craft the SETUP packet to be a valid ARM branch instruction, pointing somewhere into a temporary DFU image buffer. By first sending a payload as a partial DFU image (aborting before causing a MANIFEST), we finally get up to be able to execute 0x800 bytes of fully user controlled code.

In that payload, we send a stub which performs some runtime changes to the DFU's data structures to a) return a different product string b) overwrite an image verification vtable entry with a function that allows unsigned images. Some SRAM is carved out by this payload to store the modified vtable and custom verification function.

Nano5G Exploit Chain
--------------------

Not yet exploited, but the bootrom seems vulnerable to this bug: `control_msg(0x20, 0, 0, 255, 0)` hangs the device. Some fuzzing/bruteforcing needs to be performed to get actual code execution, though.

Nano6G, 7G Exploit Chain
------------------------

The vulnerability does not appear to exist on these devices. Either it was fixed or the USB stack has replaced with a different codebase.

License
=======

Copyright (C) 2022 Serge 'q3k' Bazanski (q3k@q3k.org)

This program is free software; you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation; either version 2 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License along
with this program; if not, write to the Free Software Foundation, Inc.,
51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.
