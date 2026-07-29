# Security Policy

## Reporting a vulnerability

Report vulnerabilities in this binding through GitHub private vulnerability
reporting, which is enabled on this repository:
<https://github.com/Haivision/srtgo/security/advisories/new>
("Security" tab -> "Report a vulnerability").

Please do not open a public issue for a suspected vulnerability. If you cannot
reach that form, open an issue that says only that you have a security report
and asks for a private channel, with no details.

Include, where you can: affected commit, the libsrt version your build actually
linked against (see "srtgo does not vendor libsrt" below for how to determine
it), OS/architecture, and a reproducer.

Fixes land on `master`. This repository publishes no tagged releases and
maintains no release branches, so consumers track a pseudo-version and should
update to the fixed commit.

## Scope

In scope: the Go/cgo code in this repository -- the SRT socket wrapper, the
epoll poll server, option handling, and the memory/lifetime boundary between Go
and C.

Not in scope: vulnerabilities in libsrt itself. Report those to the libsrt
project, <https://github.com/Haivision/srt>, not here.

## srtgo does not vendor libsrt

srtgo is a cgo binding. It contains no copy of libsrt, no submodule and no
`vendor/` directory: it compiles against the `<srt/srt.h>` headers and links
against the libsrt that *you* install on the build and target machines.
Consequently:

- A libsrt CVE cannot be fixed by a change to srtgo, and updating srtgo does
  not update libsrt.
- The libsrt actually in effect is whichever one your toolchain resolves. srtgo
  builds with a plain `#cgo LDFLAGS: -lsrt` and does not use `pkg-config`, so
  `pkg-config --modversion srt` tells you what pkg-config can see, not
  necessarily what you linked. Check your build flags and your linked binary
  (`otool -L` on macOS, `ldd` on Linux, `dumpbin /dependents` on Windows), not
  this repository. If you link libsrt statically, no such tool will show it and
  the version is fixed at link time.
- Keeping libsrt patched is the consumer's responsibility.

### Recommended minimum libsrt

**libsrt 1.5.6 or newer.** This matches the README, which states: "For
security, **srt 1.5.6 or newer is recommended**". Versions up to and including
1.5.5 are affected by two critical issues, both network-reachable without
authentication or user interaction (CVSS 3.1 base score 9.1,
`AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:H/A:H`), and both fixed in 1.5.6:

| Advisory | CVE | Issue |
| --- | --- | --- |
| [GHSA-6xg9-784j-24rm](https://github.com/Haivision/srt/security/advisories/GHSA-6xg9-784j-24rm) | CVE-2026-55869 | Buffer overflow in KMREQ/KMRSP handling |
| [GHSA-4mc6-qmpp-g7gw](https://github.com/Haivision/srt/security/advisories/GHSA-4mc6-qmpp-g7gw) | CVE-2026-55868 | Encryption state machine downgrade |

This is a different number from srtgo's minimum for *building*, which the
README gives as "srtgo requires **srt 1.4.2 or newer** to build" -- the binding
references `srt_connect_callback`, `srt_setrejectreason` and the
`SRT_EPOLLEMPTY`/`SRT_ESCLOSED`/`SRT_ESYSOBJ` error codes, none of which exist
before 1.4.2. 1.4.2 is what compiles; 1.5.6 is the floor for anything exposed
to untrusted peers.

Separately, note that an SRT stream is **unencrypted unless you configure a
passphrase**. If stream contents or integrity matter, set the `passphrase`
option on both peers and do not disable `enforcedencryption` (libsrt defaults
it to on), so a peer that offers no encryption is rejected rather than silently
accepted. Do not rely on the transport being private by default.

## Accepted risks

Known advisories that are deliberately not being acted on, and why.

### GO-2026-5024 / CVE-2026-39824 -- golang.org/x/sys/windows

Integer overflow in `NewNTUnicodeString`: a string longer than an
`NTUnicodeString` can describe is truncated instead of returning an error.
First fixed in `golang.org/x/sys` v0.44.0; this module requires v0.1.0.

Not fixed here because:

- The affected symbol is Windows-only (the Go vulnerability database records it
  under `goos: windows`) and is never called from this repository. srtgo uses
  x/sys only for address-family constants and sockaddr struct sizes, in
  `netutils_unix.go`, `netutils_windows.go` and `netutils_test.go`.
  `govulncheck` reports it as present in a required module with no call path
  into it: "your code doesn't appear to call these vulnerabilities".
- x/sys v0.44.0 declares `go 1.25.0` in its own `go.mod`. Taking it would raise
  the Go toolchain floor for every downstream consumer of srtgo, which
  currently declares `go 1.12` deliberately. That cost is judged larger than
  the risk of an unreachable Windows-only overflow.

Note that srtgo's `require golang.org/x/sys v0.1.0` is a minimum, not a pin.
Under minimal version selection, a consumer whose own module graph already
needs a newer x/sys builds against that newer version, fix included. This
decision constrains only builds of srtgo on its own.

This finding comes from `govulncheck` / the Go vulnerability database, not from
Dependabot: GitHub's advisory for CVE-2026-39824 is
[GHSA-4vpj-hr3r-4gpg](https://github.com/advisories/GHSA-4vpj-hr3r-4gpg), which
is unreviewed and carries no affected-version ranges, so it raises no Dependabot
alert here; at the time of writing this repository has no open Dependabot alerts
at all. `.github/dependabot.yml` holds `golang.org/x/sys` at v0.1.x, which
suppresses routine version-update pull requests for it and would also suppress a
security-update pull request if an alert ever did appear. That ignore rule
configures updates only; it cannot dismiss or hide an alert, which would remain
visible in this repository's Security tab.

Revisit if: srtgo starts calling `NewNTUnicodeString` (directly or through a
new x/sys API), an x/sys advisory affects a package srtgo actually imports, or
srtgo's minimum Go version is raised for other reasons -- at which point the
ignore rule should be dropped and x/sys updated.
