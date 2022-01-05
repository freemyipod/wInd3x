wInd3x
======

Nano 4G and Nano 5G bootrom/DFU exploit.

Building
--------

You'll need go, libusb, and keystone. Then:

    $ go build

Or, if you have Nix(OS), just do:

    $ nix-build

Running
-------

Put your iPod into DFU mode by connecting it over USB, holding down menu+select until it reboots, blanks the screen, then shows the Apple logo, then blanks the screen again. The iPod should enumerate as 'USB DFU Device'.

Then, run wInd3x to put the iPod into 'haxed DFU' mode. This is a modified DFU mode that allows booting any DFU image, including unsigned and unencrypted ones. The mode is temporary, and will be active only until next (re)boot, the exploit does not modify the device permanently in any way.

    $ ./wInd3x
    2021/12/31 00:59:13 wInd3x - iPod Nano 4G and Nano 5G bootrom exploit
    ...
    2021/12/31 00:59:15 Device will now accept signed and unsigned DFU images.

You can then use any DFU tool to upload any DFU image and the device should boot it. You can also pass the -image argument to wInd3x to make it immediately send a file as a DFU image after running haxed DFU mode.

Haxed DFU Mode
--------------

Once in haxed DFU mode, the DFU will continue as previously, and you will still be able to send properly signed and encrypted images (like WTF). However, signature checking (in header and footer) is disabled. What this means:

 - Images with format '3' (like WTF) will not be sigchecked, but will be decrypted.
 - Images with format '4' will not be sigchecked and will not be decrypted.
 - Pwnage 2.0 images *might* work if they are built to be able to run without having to exploit footer signature checking.


To make your own DFU images, you should thus make format '4' images, not encrypt them and not sign them.

TODO: provide tool to generate images

Running EmCORE
--------------

To be documented.

Vulnerability
=============

This exploits a vulnerability in the standard SETUP packet parsing code of the bootrom, in which the wIndex parameter is not checked for bmRequest == {0x20, 0x40}, but is still used to index an array of interface/class handlers (that in the Bootrom has a length of 1).

Nano 4G and 5G Exploit Chain
--------------------

The first requirement is to find a suitable (blx r0) instruction in the bootrom code of the device. For Nano 4G the only one such instruction is at offset 0x3b0, and for Nano 5G there is such instruction at 0x37c. We'll refer to it as X below.

We abuse the fact that wIndex == 3 for bmRequest 0x40 treats a 'bytes left to sent over USB' counter as a function pointer and calls it with r0 == address of SETUP. We massage the DFU mode into attempting to send us X+0x40 bytes, and failing after 0x40 bytes, thereby leaving the counter at X bytes and executing code at address X.

Since the bootrom is mapped at offset 0x0 as well as 0x20000000 at boot, this means we execute bootrom code, and X happens to point to a 'blx r0' instruction. This in turn causes the CPU to interpret the SETUP packet received as ARM code, because the SETUP handler is called with the SETUP packet as its argument, i.e. r0.

We specially craft the SETUP packet to be a valid ARM branch instruction, pointing somewhere into a temporary DFU image buffer. By first sending a payload as a partial DFU image (aborting before causing a MANIFEST), we finally get up to be able to execute either 0x800 on Nano 4G or 0x400 on Nano 5G bytes of fully user controlled code.

In that payload, we send a stub which performs some runtime changes to the DFU's data structures to a) return a different product string b) overwrite an image verification vtable entry with a function that allows unsigned images. Some SRAM is carved out by this payload to store the modified vtable and custom verification function.

Nano 6G and 7G Exploit Chain
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
