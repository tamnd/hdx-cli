---
title: "Installation"
description: "Install hdx from a release, with go install, or from source."
weight: 20
---

## Prebuilt binaries

Every [release](https://github.com/tamnd/hdx-cli/releases) carries archives for Linux, macOS,
and Windows on amd64 and arm64, plus deb, rpm, and apk packages for Linux.
Download, unpack, put `hdx` on your `PATH`, done. The `checksums.txt`
on each release is signed with keyless [cosign](https://docs.sigstore.dev/) if
you want to verify before running.

## With Go

```bash
go install github.com/tamnd/hdx-cli/cmd/hdx@latest
```

That puts `hdx` in `$(go env GOPATH)/bin`, which is `~/go/bin` unless
you moved it. Make sure that directory is on your `PATH`.

## From source

```bash
git clone https://github.com/tamnd/hdx-cli
cd hdx-cli
make build        # produces ./bin/hdx
./bin/hdx version
```

## Container image

```bash
docker run --rm ghcr.io/tamnd/hdx:latest --help
```

## Checking the install

```bash
hdx version
```

prints the version and exits.
