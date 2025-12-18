# opensnell

This repository is a fork of [open-snell](https://github.com/icpz/open-snell).

## Quick install (systemd)

```sh
curl -fsSL https://raw.githubusercontent.com/noodlesandoa-oss/opensnell/main/install.sh | sudo bash
```

- Requires: `systemd` (and `go` only if it falls back to source build)
- Config: `/etc/open-snell/snell-server.conf`
- Service: `snell-server.service`
- Logs: `journalctl -u snell-server.service -f`

### Logging

This project uses `glog`.

- By default it now logs to stderr (so `journalctl` can see it).
- You can enable more verbose debug logs with either:
	- CLI flag: `--verbose` (equivalent to `-v=1`)
	- Config key (systemd install): add `verbose = true` under `[snell-server]` or `[snell-client]`

The installer prefers GitHub Release assets named like `open-snell-vX.Y.Z-linux-amd64.tar.gz` (and `arm64`).
For `releases/latest/download` compatibility, releases also include stable asset names like `open-snell-linux-amd64.tar.gz`.

## License

This project is licensed under the GNU General Public License v3.0, same as the original repository. See [LICENSE.md](LICENSE.md) for details.

## Description

An open source port of [snell](https://github.com/surge-networks/snell).

See the original repository for more details on build and usage.
