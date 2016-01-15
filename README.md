# netns

[![Circle CI](https://circleci.com/gh/jfrazelle/netns.svg?style=svg)](https://circleci.com/gh/jfrazelle/netns)

Runc hook for setting up default bridge networking.

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
  -ip string
        ip address (default "10.100.55.1/24")
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
