# HiDPI backend findings

Research date: 2026-11-05. Read-only investigation; no external script was run and no system display configuration was changed.

## MonitorBar verification on the connected device

The subsequent implementation was checked on the M4 Mac mini and H27D9 over DisplayPort. The panel EDID reports 2560×1440 and 600×330 mm. CoreGraphics initially exposed 2048×1152 HiDPI / 4096×2304 backing as the largest usable HiDPI desktop. CGS additionally exposed native-equivalent 2560×1440 HiDPI / 5120×2880 backing at 100 Hz. Configuring that CGS mode succeeded and the actual CoreGraphics current mode confirmed those dimensions and rate.

MonitorBar reenumerates modes before switching, rather than retaining mode references across display configuration changes. `MONITORBAR_LIVE_HIDPI=1 go test -v ./internal/display -run '^TestLiveHiDPI$'` checks both 2048×1152 and 2560×1440 at the current refresh, then restores the initial mode. This test changes the real screen and is skipped by default.

2304×1296 and 2496×1404 were absent from this device's initial list. The override writer is tested against temporary files, including preservation, repeated installation, exact restoration, and refusal to overwrite later external changes. Their activation after a real display reconnect remains unverified. No system override was required for the working 2560×1440 mode. A virtual display backend is not part of this implementation.

## Recommended implementation order

For the M4 mini / H27D9 2560×1440 DisplayPort setup, first enumerate both public CG and private CGS modes. A public maximum of logical 2048×1152 / backing 4096×2304 does **not** establish a hardware limit: CG hides some HiDPI variants. Select an existing suitable mode before creating anything. If the desired modes are absent, a virtual display mirrored to the physical display avoids installing a root-owned override and needing a physical reconnect. Native custom override injection is another backend; its installed bytes are a request to macOS, not proof of an available mode.

Current BetterDisplay documentation says limits vary with Mac, connection, arrangement and macOS; a custom definition cannot remove them. Do not infer an M4-wide 4096 backing limit from this user's public mode list. [Current limits guidance](https://betterdisplay.pro/guide/faq/scaling/hidpi-resolution-limits/).

## Existing hidden modes: exact CGS ABI

Crisp uses these private functions, declared in [Crisp-Bridging-Header.h](https://github.com/didriksg/Crisp/blob/main/Crisp/Crisp-Bridging-Header.h):

```c
CGError CGSGetNumberOfDisplayModes(uint32_t display, int32_t *count);
CGError CGSGetDisplayModeDescriptionOfLength(uint32_t display, int32_t index,
                                             void *description, int32_t length);
CGError CGSConfigureDisplayMode(CGDisplayConfigRef config, uint32_t display,
                               int32_t modeNumber);
```

The description is 212 bytes: uint32 modeNumber at offset 0; uint32 flags at 4; uint32 logical width at 8; uint32 logical height at 12; uint32 depth at 16; 168 reserved bytes at 20; uint16 reserved at 188; uint16 refresh in Hz at 190; 16 reserved bytes at 192; float32 density at 208. Pass length 212. Backing dimensions are rounded logical dimensions multiplied by density. Crisp excludes flags `0x40000000` (unusable) and only imports density ≥1.5 modes with matching aspect ratio. It merges by mode number with public modes. [Enumeration implementation](https://github.com/didriksg/Crisp/blob/main/Crisp/Models/DisplayMode.swift).

Apply a CGS-only mode with `CGBeginDisplayConfiguration(&config)`, `CGSConfigureDisplayMode(config, display, modeNumber)`, then `CGCompleteDisplayConfiguration(config, scope)`. The first argument is the configuration token, **not** `CGSMainConnectionID`; Crisp reports the latter crashes on macOS 26. Cancel the transaction if configuring fails. Mode IDs can change after reenumeration, so save dimensions, backing and refresh instead of an ID. Commit propagation can take about 1.3 seconds after reconnect; verify the current mode for up to 3 seconds. These APIs select existing records; they do not create missing modes. [Apply/verify implementation](https://github.com/didriksg/Crisp/blob/main/Crisp/Services/ResolutionService.swift).

## Virtual fallback: exact configuration

Use private Objective-C `CGVirtualDisplayDescriptor`, `CGVirtualDisplayMode`, `CGVirtualDisplaySettings`, `CGVirtualDisplay`. Their selectors and property declarations are in the [Crisp header](https://github.com/didriksg/Crisp/blob/main/Crisp/Crisp-Bridging-Header.h). This can be called through the existing purego Objective-C bridge; no Swift helper is inherently necessary.

For requested logical 2560×1440:

1. On the main thread create a descriptor with nonzero vendor ID, stable product/serial identity, name, physical size in millimeters, `maxPixelsWide=5120`, `maxPixelsHigh=2880`. Create `CGVirtualDisplay` using `initWithDescriptor:` and retain it strongly.
2. Set settings `hiDPI=true`; provide both mode dimensions **5120×2880 and 2560×1440** at the desired rate. Add analogous backing/logical pairs for 2496×1404 and 2304×1296. `CGVirtualDisplayMode` selector is `initWithWidth:height:refreshRate:`. Crisp found backing-only declarations enumerate but fail selection; both halves are required for its working mirror implementation.
3. Call `applySettings:` off main because it can block on WindowServer. Wait for a nonzero displayID and mode enumeration. Select the virtual public CG mode with logical 2560×1440 and backing 5120×2880. Require exact backing dimensions; selecting a same-size LoDPI row defeats the goal.
4. Mirror **physical target to virtual source** using `CGConfigureDisplayMirrorOfDisplay(config, physicalID, virtualID)`. Verify `CGDisplayMirrorsDisplay(physicalID)==virtualID`, and verify the virtual source's active backing.
5. Restore by unmirroring the physical target with source 0 before releasing the virtual object. Keep the object if unmirroring fails. Store the original physical mode and restore it.

Source: [Crisp MirroredModeService.swift](https://github.com/didriksg/Crisp/blob/main/Crisp/Services/MirroredModeService.swift). Its published `beyondCapStops` deliberately limits automatic use to ultrawides until other panel shapes are verified. Thus this is a concrete implementation reference, not verification for this 16:9 H27D9 or for 100 Hz. Source creation generally uses the physical panel's current refresh. Registration may show a native macOS display picker. A process lifetime virtual display needs recreation after restart; quit/error cleanup must unmirror first.

BetterDisplay describes virtual mirroring as a workaround with different performance and sleep/wake behavior; verify text, refresh and recovery on the user's real setup. [Virtual workflow](https://betterdisplay.pro/guide/tutorials/fully-scalable-hidpi-desktop/).

## Native override byte format

Path: `/Library/Displays/Contents/Resources/Overrides/DisplayVendorID-<lowercase hex>/DisplayProductID-<lowercase hex>` (no extension). Plist `scale-resolutions` is an array of **data**, not strings. Each integer below is unsigned 32-bit **big endian**.

| Format | Words | Primary implementation |
|---|---|---|
| 8 bytes | backingWidth, backingHeight | Current Crisp |
| 12 bytes | backingWidth, backingHeight, 1 | one-key-hidpi `create_res_3` |
| 16 bytes | backingWidth, backingHeight, 1, 0x00200000 | one-key-hidpi `create_res_2` |
| 16 bytes | backingWidth, backingHeight, 9, 0x00200000 | one-key-hidpi `create_res_4`; kirakirag/hidpi |

Sources: [Crisp encoder](https://github.com/didriksg/Crisp/blob/main/Crisp/Services/HiDPIService.swift), [one-key-hidpi encoders](https://github.com/xzhih/one-key-hidpi/blob/master/hidpi.sh), [kirakirag implementation](https://github.com/kirakirag/hidpi/blob/main/hidpi). These are empirical private formats, not an Apple-supported schema. Do not invent meanings for all bits or assume every form behaves identically. A 16-byte flagged mode may be hidden from public CG enumeration, which is a reason to retain the CGS enumeration path.

Exact 8-byte data for the requested modes:

| Logical | Backing | Hex | Base64 |
|---|---|---|---|
| 2560×1440 | 5120×2880 | `0000140000000b40` | `AAAUAAAAC0A=` |
| 2496×1404 | 4992×2808 | `0000138000000af8` | `AAATgAAACvg=` |
| 2304×1296 | 4608×2592 | `0000120000000a20` | `AAASAAAACiA=` |

A minimal new Crisp-style plist is:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>scale-resolutions</key><array>
    <data>AAAUAAAAC0A=</data>
    <data>AAATgAAACvg=</data>
    <data>AAASAAAACiA=</data>
  </array>
</dict></plist>
```

Current Crisp does not require EDID edits, fake native dimensions, product-name changes, `target-default-ppmm`, `default-resolution`, or `DisplayResolutionEnabled` for this injection path. Legacy one-key-hidpi and kirakirag also set `DisplayResolutionEnabled`; their doing so does not establish necessity on current macOS. Do not clear an existing global flag on uninstall if this application did not own it. Existing working HiDPI also gives no reason to enable a global flag preemptively.

## Installation, preservation and reload

`/Library/Displays` installation needs administrator authorization. Crisp executes `do shell script ... with administrator privileges`. Its current helper builds an XML plist in memory, embeds validated Base64 bytes in the privileged command, creates a staging file in the root-owned destination directory, checks it using `plutil`, chmods 0644, and atomically renames it. This avoids reading a mutable user-owned source path as root. [HiDPIOverrideCommand.swift](https://github.com/didriksg/Crisp/blob/main/Crisp/Models/HiDPIOverrideCommand.swift).

Crisp replaces an entire existing override and removes the entire file when disabling. **Do not copy that behavior into MonitorBar**: preserve all original dictionary keys and existing `scale-resolutions` entries, append only missing byte entries, keep a full exact original backup, and undo only entries owned by MonitorBar unless restoring an unchanged post-install file. Read/recheck/merge under the privileged operation to avoid clobbering concurrent edits. The override applies to all monitors with that vendor/product identity, not just one serial number. [BetterDisplay model scope](https://betterdisplay.pro/guide/interface-reference/settings/displays/display/system-configuration/).

`IOServiceRequestProbe(service,0)` in Crisp's older-style HiDPI service is insufficient to activate an installed override. Its **current measured notes** say both that probe and `SLSDetectDisplays` are no-ops here; disabling and re-enabling the framebuffer rereads it. The relevant API is private `SLSConfigureDisplayEnabled(config, display, bool)`, inside ordinary CG begin/complete transactions; disabled display recovery enumerates `SLSGetDisplayList`, since public online lists omit disabled displays. The screen blanks. Portable Macs may sleep if the only display is disabled; Crisp temporarily keeps a virtual display alive to prevent that. [Measured softReconnect notes](https://github.com/didriksg/Crisp/blob/main/docs/display-notes.md).

For a minimal override backend, report “installed; reconnect or restart required” and reenumerate after actual reconnect/restart instead of claiming success from probe return codes. On this single-monitor mini, virtual fallback avoids implementing that off/on recovery loop. A loaded override still needs actual mode selection and backing verification.

## Exact-native resolution caveat

Older BetterDisplay wiki says the native physical size was normally LoDPI-only and recommends virtual mirroring for exact-native/super-native logical HiDPI. The maintainer later noted this restriction changed across macOS and models; current Crisp explicitly observes and applies hidden native-equivalent HiDPI modes on some hardware. Accordingly, **neither “always impossible” nor “always works” is supported**. Try logical2560/backing5120 first if enumerated; otherwise native injection may or may not yield it, while near-native2496/backing4992 and2304/backing4608 are sensible requests, not guaranteed outcomes.

Sources: [historical wiki](https://github.com/waydabber/BetterDisplay/wiki/Fully-scalable-HiDPI-desktop/198c1dd3cedb1f056e664faaaeb6af608ae001a2), [maintainer native-equivalent discussion](https://github.com/waydabber/BetterDisplay/discussions/3949), [current Crisp restore notes](https://github.com/didriksg/Crisp/blob/main/docs/display-notes.md). No source inspected verifies this exact M4/H27D9/DP configuration. A successful native or virtual result must be read back as the intended logical size and twice the backing size, plus the actual refresh, before presenting it as working.
