# netns

[![make-all](https://github.com/genuinetools/netns/workflows/make%20all/badge.svg)](https://github.com/genuinetools/netns/actions?query=workflow%3A%22make+all%22)
[![make-image](https://github.com/genuinetools/netns/workflows/make%20image/badge.svg)](https://github.com/genuinetools/netns/actions?query=workflow%3A%22make+image%22)
[![GoDoc](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=for-the-badge)](https://godoc.org/github.com/genuinetools/netns)
[![Github All Releases](https://img.shields.io/github/downloads/genuinetools/netns/total.svg?style=for-the-badge)](https://github.com/genuinetools/netns/releases)

Runc hook for setting up default bridge networking.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**

- [Installation](#installation)
    - [Binaries](#binaries)
    - [Via Go](#via-go)
- [Usage](#usage)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Installation

#### Binaries

For installation instructions from binaries please visit the [Releases Page](https://github.com/genuinetools/netns/releases).

#### Via Go

```console
$ go get github.com/genuinetools/netns
```

## Usage

```console
$ netns -h
netns -  Runc hook for setting up default bridge networking.

Usage: netns <command>

Flags:

  --ipfile     file in which to save the containers ip address (default: .ip)
  --mtu        mtu for bridge (default: 1500)
  --state-dir  directory for saving state, used for ip allocation (default: /run/github.com/genuinetools/netns)
  --bridge     name for bridge (default: netns0)
  -d           enable debug logging (default: false)
  --iface      name of interface in the namespace (default: eth0)
  --ip         ip address for bridge (default: 172.19.0.1/16)
  --static-ip  Enable static IP Address (default: <none>)

Commands:

  create   Create a network.
  ls       List networks.
  rm       Delete a network.
  version  Show the version information.
```

Place this in the `Hooks.CreateRuntime` field of your `runc` config.

```json
{
    ...
    "hooks": {
        "createRuntime": [
            {
                "path": "/path/to/netns"
            }
        ]
    },
    ...
}
```

**List network namespaces**

```console
$ sudo netns ls
IP                  LOCAL VETH          PID                 STATUS
172.19.0.3          netnsv0-21635       21635               running
172.19.0.4          netnsv0-21835       21835               running
172.19.0.5          netnsv0-22094       22094               running
172.19.0.6          netnsv0-25996       25996               running
```
