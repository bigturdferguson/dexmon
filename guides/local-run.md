# Running dexmon Locally (Testing Only)

> **This is for local testing only.** There is no persistent storage — the database is lost when the process stops — and there is no public URL for emergency alarm callbacks. For production use, [deploy to Fly.io](../README.md#deploy-to-flyio).

---

## Build

```bash
go build -o dexmon .
```

The binary has no runtime dependencies.

---

## Set environment variables

Set one variable per credential. The names must match the `${VAR}` placeholders in your `config.toml` exactly.

```bash
export PUSHOVER_APP_TOKEN=your_app_token_here
export PUSHOVER_USER_KEY_BRANDON=your_user_key_here   # one per recipient
export DEXCOM_USER_NOAH=noah@example.com
export DEXCOM_PASS_NOAH=dexcom_password_here
export HEALTHCHECKS_PING_URL=https://hc-ping.com/...  # optional
```

| Variable | Required | Description |
|---|---|---|
| `PUSHOVER_APP_TOKEN` | Yes | Pushover application token — pushover.net → Your Applications → API Token/Key |
| `PUSHOVER_USER_KEY_<NAME>` | Per recipient | Pushover user key — pushover.net → User Key (top of page after login) |
| `DEXCOM_USER_<NAME>` | Per account | Dexcom login email |
| `DEXCOM_PASS_<NAME>` | Per account | Dexcom login password |
| `HEALTHCHECKS_PING_URL` | No | Watchdog ping URL; leave unset to disable |

The `<NAME>` suffix must match the placeholder in `config.toml`. If your config has `${DEXCOM_USER_NOAH}`, set `DEXCOM_USER_NOAH`.

---

## Run

```bash
./dexmon -config config.toml -db dexmon.db
```

| Flag | Default | Description |
|---|---|---|
| `-config` | `config.toml` | Path to the TOML config file |
| `-db` | `dexmon.db` | Path to the SQLite database file |

The process logs to stdout. Press Ctrl+C to stop. The database file (`dexmon.db`) is created in the current directory on first run.
