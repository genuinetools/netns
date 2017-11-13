# netns

[![Travis CI](https://travis-ci.org/jessfraz/netns.svg?branch=master)](https://travis-ci.org/jessfraz/netns)

Runc hook for setting up default bridge networking.

## Installation

#### Binaries

- **linux** [386](https://github.com/jessfraz/netns/releases/download/v0.2.2/netns-linux-386) / [amd64](https://github.com/jessfraz/netns/releases/download/v0.2.2/netns-linux-amd64) / [arm](https://github.com/jessfraz/netns/releases/download/v0.2.2/netns-linux-arm) / [arm64](https://github.com/jessfraz/netns/releases/download/v0.2.2/netns-linux-arm64)

#### Via Go

```bash
$ go get github.com/jessfraz/netns
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
 Version: v0.1.0

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
172.19.0.5          netnsv0.2.294       22094               running
172.19.0.6          netnsv0-25996       25996               running
```

