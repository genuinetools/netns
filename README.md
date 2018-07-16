# netns

[![Travis CI](https://img.shields.io/travis/genuinetools/netns.svg?style=for-the-badge)](https://travis-ci.org/genuinetools/netns)
[![GoDoc](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=for-the-badge)](https://godoc.org/github.com/genuinetools/netns)
[![Github All Releases](https://img.shields.io/github/downloads/genuinetools/netns/total.svg?style=for-the-badge)](https://github.com/genuinetools/netns/releases)

Runc hook for setting up default bridge networking.

## Installation

#### Binaries

For installation instructions from binaries please visit the [Releases Page](https://github.com/genuinetools/netns/releases).

- **linux** [386](https://github.com/genuinetools/netns/releases/download/v0.4.4/netns-linux-386) / [amd64](https://github.com/genuinetools/netns/releases/download/v0.4.4/netns-linux-amd64) / [arm](https://github.com/genuinetools/netns/releases/download/v0.4.4/netns-linux-arm) / [arm64](https://github.com/genuinetools/netns/releases/download/v0.4.4/netns-linux-arm64)

#### Via Go

```bash
$ go get github.com/genuinetools/netns
```

## Usage

```console
$ netns --help
            _
 _ __   ___| |_ _ __  ___
| '_ \ / _ \ __| '_ \/ __|
| | | |  __/ |_| | | \__ \
|_| |_|\___|\__|_| |_|___/

 Runc hook for setting up default bridge networking.
 Version: v0.4.4

 Netns provides the following commands. Usage format:

    netns [-flag value] [-flag value] command

  Where command is one of:

    createbr, delbr, [ls|list], delete

  If command is blank (e.g. when called via a hook) it
  will create a network endpoint in the expected net
  namespace details for that PID.

  -bridge string
        name for bridge (default "netns0")
  -d    run in debug mode
  -iface string
        name of interface in the namespace (default "eth0")
  -ip string
        ip address for bridge (default "172.19.0.1/16")
  -ipfile string
        file in which to save the containers ip address (default ".ip")
  -mtu int
        mtu for bridge (default 1500)
  -state-dir string
        directory for saving state, used for ip allocation (default "/run/github.com/genuinetools/netns")
  -v    print version and exit (shorthand)
  -version
        print version and exit
```

Place this in the `Hooks.Prestart` field of your `runc` config.

```json
{
    ...
    "hooks": {
        "prestart": [
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

