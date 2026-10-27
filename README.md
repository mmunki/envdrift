# envdrift

**Explain why a command works on your machine and fails in CI.**

`envdrift` captures a portable, secret-redacted fingerprint of a development or
CI environment, then explains the differences between two fingerprints in plain
language. It is one static binary with no dependencies, no daemon and no
telemetry.

## The problem

"It works on my machine" is still one of the most expensive sentences in
software. The cause is rarely the source code: it is a different compiler, a
missing tool, a PATH entry, an environment variable, a resource limit, a locale
or a dirty checkout. People compare screenshots and guess.

`envdrift` makes that comparison mechanical.

A typical report (abridged):

```text
$ envdrift diff ci.json local.json
envdrift diff
  left:  linux/amd64, Ubuntu 24.04, main@a1b2c3d
  right: darwin/arm64, macOS 15.1, main@a1b2c3d

CRITICAL (3)
  host.arch                      amd64 -> arm64
      The CPU architecture differs. A binary built for one architecture cannot run on the other.
  host.os                        linux -> darwin
      The operating system differs. Native binaries, path separators, shell syntax and syscalls are not interchangeable.
  tool.gcc                       gcc (Ubuntu 13.3.0) 13.3.0 -> not found
      gcc is installed on one side but missing on the other. A command that depends on it cannot run there.

WARNING (2)
  env.CC                         gcc -> clang
      CC directly changes how the toolchain resolves compilers, libraries or runtimes.
  path.PATH                      /usr/local/bin:/usr/bin:/bin -> /opt/homebrew/bin:/usr/bin:/bin
      PATH resolves to different directories. The same command name can run a different binary.

Summary: 3 critical, 2 warning, 0 info
```

## Install

Download the archive for your platform from the
[releases](https://github.com/mmunki/envdrift/releases), unpack it and put the
binary on `PATH`. Every release ships:

| Platform | Archive |
| --- | --- |
| Linux x86-64 | `envdrift-linux-amd64.tar.gz` |
| Linux arm64 | `envdrift-linux-arm64.tar.gz` |
| macOS Intel | `envdrift-darwin-amd64.tar.gz` |
| macOS Apple Silicon | `envdrift-darwin-arm64.tar.gz` |
| Windows x86-64 | `envdrift-windows-amd64.zip` |
| Windows arm64 | `envdrift-windows-arm64.zip` |

The release also contains `checksums.txt` and the full source archive. The
SHA-256 of every asset is written into the release notes.

Or build it yourself:

```bash
go build -o envdrift ./cmd/envdrift
```

## Use it

Capture the environment where something works:

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

Use `--format markdown` to paste the result into an issue or a job summary, or
`--format json` for tooling. `--fail-on critical` (or `warning`, `info`,
`never`) controls the exit code, so the command can be used as a CI gate.

### What is captured

- OS, architecture, distribution, kernel, shell, locale, timezone, terminal,
  CPU count and memory.
- Versions and paths of common toolchains: Go, Node, npm, Python, Java,
  Rust, GCC, Clang, Make, CMake, Ninja, Docker, Git, Bash, Zsh, Deno, Bun,
  Ruby, PHP and .NET.
- A curated set of environment variables that change build behaviour (`CC`,
  `CFLAGS`, `JAVA_HOME`, `GOROOT`, `NODE_OPTIONS`, `PYTHONPATH`, `SDKROOT`,
  `ANDROID_HOME`, `GITHUB_*`, `RUNNER_*` and friends).
- `PATH` entries in order.
- Resource limits (`nofile`, `nproc` on Linux and macOS).
- Git branch, short commit and whether the tree is dirty.
- Optional free-form labels with `--label key=value`.

Use `--all-env` when the failing variable is a custom one that is not in the
curated list.

### Secrets never leave the machine

Redaction happens before anything is written. A variable whose name contains a
secret-looking segment (`TOKEN`, `SECRET`, `PASSWORD`, `KEY`, `CREDENTIAL`,
`AUTH`, `SESSION`, `COOKIE`, `PAT`, ...) is replaced as a whole. Values that
look like GitHub, OpenAI, AWS, Slack, Google or private-key material are
replaced even when the variable name looks harmless, including credentials in
URLs, `Bearer` headers and `?token=` query parameters.

The placeholder is `<redacted>`. The capture also lists which variable names
were redacted, never their values. There is no option to disable redaction.

A fingerprint is still a snapshot of a machine. Review it before sharing it
publicly; `--all-env` can include project names, paths and other metadata that
is not a secret but may be private.

## CI integration

`envdrift snippet` prints a ready-to-paste GitHub Actions step:

```yaml
- name: Capture environment fingerprint
  run: envdrift capture --out envdrift-ci.json

- name: Upload environment fingerprint
  uses: actions/upload-artifact@v4
  with:
    name: envdrift-${{ runner.os }}-${{ runner.arch }}
    path: envdrift-ci.json
    if-no-files-found: error
```

When a job fails, download the artifact, capture your own machine and run
`envdrift diff`. The report ranks the differences and tells you which ones can
actually change the result.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | No difference at or above `--fail-on` (default: `warning`). |
| `1` | A difference at or above `--fail-on` exists. |
| `2` | Usage error, unreadable file or unsupported fingerprint schema. |

## Design

- **Local first.** Capture and diff run entirely offline. Nothing is uploaded.
- **Explain, do not dump.** Every difference has a severity and a sentence
  describing why it can matter. A raw diff of environment variables is not
  useful on its own.
- **Safe by default.** Only a curated environment allowlist is captured, and
  redaction cannot be turned off.
- **Small.** The standard library only, one static binary, six release targets.
- **Versioned format.** Fingerprints carry `schema_version`; a newer format is
  rejected with a clear message instead of being misread.

A fingerprint is not a sandbox or a lockfile. It describes the machine, not the
project's dependency graph, and tool versions are best-effort (some tools print
nothing useful). It is the fastest way to turn a vague "works on my machine"
report into a concrete, ranked list of suspects.

## Development

```bash
go test ./...
go vet ./...
bash scripts/build.sh
```

The test suite covers redaction, capture, comparison, severity ordering, report
rendering and the CLI. CI runs the suite natively on Linux, macOS and Windows,
cross-builds and verifies every release target, and packages the archives:

| Target | Build | Archive |
| --- | --- | --- |
| linux/amd64 | yes | `envdrift-linux-amd64.tar.gz` |
| linux/arm64 | yes | `envdrift-linux-arm64.tar.gz` |
| darwin/amd64 | yes | `envdrift-darwin-amd64.tar.gz` |
| darwin/arm64 | yes | `envdrift-darwin-arm64.tar.gz` |
| windows/amd64 | yes | `envdrift-windows-amd64.zip` |
| windows/arm64 | yes | `envdrift-windows-arm64.zip` |

## License

MIT. See [LICENSE](LICENSE).
