# Go-Lancache

A web proxy and file storage system written in Go.

Go-Lancache is designed for local networks where multiple devices may request the same files, such as game updates, software packages, or other large files.

It saves eligible files to local storage and serves subsequent requests from the local cache instead of downloading the same files from the internet again.

This is an alternative implementation inspired by the [Lancache](https://lancache.net/) project and my previous [Lancache Storage](https://github.com/DjoSmer/lancache-storage) implementation.

## How it works

```text
Client 1 ──┐
Client 2 ──┤
Client 3 ──┼──> Go-Lancache ──> Internet
Client 4 ──┘          │
                      ▼
                  Local Storage
```

## Status

**Work in progress — not finished yet.**
