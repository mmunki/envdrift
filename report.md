# envdrift — what it is and what it is for

## What it is

`envdrift` is a small, dependency-free command-line tool that explains why a
command behaves differently on two machines. It captures a portable,
secret-redacted **fingerprint** of an environment, compares two fingerprints,
and reports every difference with a severity and a plain-language explanation
of why the difference can change the result.

It is a single static Go binary. It runs entirely offline: no daemon, no
account, no network calls and no telemetry.

## What problem it solves

"It works on my machine" is usually not about the source code. The cause is a
different OS or CPU architecture, a missing compiler, a different toolchain
version, a `PATH` entry, a build-related environment variable, a resource
limit, a locale, or a dirty checkout. People compare screenshots and guess.

`envdrift` turns that guesswork into a ranked list of concrete suspects and
tells the reader which differences are likely to matter.

## How it is used

Capture the environment where the command works:

```bash
envdrift capture --out local.json
```

Capture the environment where it fails, for example in CI:

```bash
envdrift capture --out ci.json
```

Compare the two:

```bash
envdrift diff ci.json local.json
```

The text report groups differences into **critical**, **warning** and **info**
and prints a sentence of impact for each one. The same comparison can be
rendered for an issue or a job summary, or consumed by tooling:

```bash
envdrift diff --format markdown ci.json local.json
envdrift diff --format json ci.json local.json
```

`--fail-on critical` (or `warning`, `info`, `never`) sets the exit threshold,
so a diff can act as a CI gate. `envdrift snippet` prints a ready-to-paste
GitHub Actions step that captures and uploads a fingerprint with every run.

## What a fingerprint contains

- Host: OS, architecture, distribution, kernel, shell, locale, timezone,
  terminal, CPU count and memory.
- Toolchains: versions and paths for Go, Node, npm, Python, Java, Rust, GCC,
  Clang, Make, CMake, Ninja, Docker, Git, Bash, Zsh, Deno, Bun, Ruby, PHP and
  .NET.
- A curated allowlist of environment variables that change build behaviour
  (`CC`, `CFLAGS`, `JAVA_HOME`, `GOROOT`, `NODE_OPTIONS`, `PYTHONPATH`,
  `SDKROOT`, `ANDROID_HOME`, `GITHUB_*`, `RUNNER_*` and similar).
- `PATH` entries in order.
- Resource limits (`nofile`, `nproc` on Linux and macOS).
- Git branch, short commit and whether the working tree is dirty.
- Optional free-form labels added with `--label key=value`.
- `--all-env` extends the capture to every environment variable when the
  culprit is a custom one.

## Secret safety

Redaction happens before anything is written to disk. Variables whose names
look secret (`TOKEN`, `SECRET`, `PASSWORD`, `KEY`, `CREDENTIAL`, `AUTH`,
`SESSION`, `COOKIE`, `PAT`, ...) are replaced as a whole. Values that match
GitHub, OpenAI, AWS, Slack, Google or private-key material are replaced even
when the variable name looks harmless, including credentials in URLs,
`Bearer` headers and `?token=` query parameters. The placeholder is
`<redacted>`, and redaction cannot be disabled. A fingerprint is still a
snapshot of a machine, so it should be reviewed before being shared publicly.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | No difference at or above `--fail-on` (default: `warning`). |
| `1` | A difference at or above `--fail-on` exists. |
| `2` | Usage error, unreadable file or unsupported fingerprint schema. |

## Where it fits

`envdrift` is a debugging aid, not a lockfile, a container or a sandbox. It
describes the machine, not the project's dependency graph, and tool version
detection is best-effort because some tools print nothing useful. Its purpose
is to shorten the distance between a vague "works on my machine" report and a
specific, checkable difference.

## Install

Download the archive for the target platform from the releases page, unpack
it and put the binary on `PATH`. Supported targets are Linux, macOS and
Windows on amd64 and arm64, with `checksums.txt` and a source archive shipped
alongside. To build from source:

```bash
go build -o envdrift ./cmd/envdrift
```

License: MIT.
