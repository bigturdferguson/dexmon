# Deploying dexmon on Fly.io

This guide walks through deploying dexmon to Fly.io — a cloud platform that runs containers. You get a persistent volume for the SQLite database, an automatic HTTPS URL for Pushover callbacks, and a machine that runs 24/7. The free tier is sufficient.

---

## Prerequisites

- **flyctl** installed:
  ```bash
  curl -L https://fly.io/install.sh | sh
  ```
- A **Fly.io account** at [fly.io](https://fly.io) (free, no credit card required for this use)
- A **Pushover** account at pushover.net with an application created
- **Dexcom Share** enabled on the patient's Dexcom G-series app (Settings → Share → Invite Followers)

---

## Get your credentials

**Pushover app token**
Go to [pushover.net](https://pushover.net) → scroll to **Your Applications** → click your app → copy the **API Token/Key**. This is `PUSHOVER_APP_TOKEN`.

**Pushover user key**
On [pushover.net](https://pushover.net), your **User Key** is shown at the top of the page after logging in. Each person who receives notifications needs their own user key.

**Dexcom Share credentials**
The email address and password used to log in to the Dexcom mobile app.

---

## Prepare your config.toml

Copy the example and fill in your alarm rules:

```bash
cp config.toml.example config.toml
```

Edit `config.toml`:

- Fill in `[recipients]` with names and `${PUSHOVER_USER_KEY_*}` placeholders
- Fill in `[accounts]` with the patient name and `${DEXCOM_USER_*}` / `${DEXCOM_PASS_*}` placeholders
- Set alarm thresholds under `[[accounts.<name>.alarms]]`
- Leave `callback_url = ""` for now — you get the real URL after the first deploy

**Do not put actual credentials in `config.toml`.** The `${VAR}` placeholders stay as-is. The deploy script prompts for the values and uploads them to Fly's encrypted secret store.

---

## Run `./fly/deploy.sh`

From the project root:

```bash
./fly/deploy.sh
```

The script walks you through each step. Here is what each prompt means:

**App name**
A globally unique name across all Fly.io users. `dexmon` is almost certainly taken — use something like `dexmon-noah` or `dexmon-yourname`. This becomes your app's URL: `https://<appname>.fly.dev`.

**Region**
The Fly.io region closest to you. Common options:

| Code | Location |
|------|----------|
| `iad` | Northern Virginia (US East) |
| `ord` | Chicago (US Central) |
| `lax` | Los Angeles (US West) |
| `lhr` | London (Europe) |
| `syd` | Sydney (Australia) |

Full list: [fly.io/docs/reference/regions](https://fly.io/docs/reference/regions/)

**Secret prompts**
The script scans your `config.toml` for `${VAR}` references and prompts for each one. Type the actual value when prompted — input is hidden. Values go directly to Fly's encrypted secret store and never touch your filesystem.

After all prompts, the script:
1. Creates the Fly app
2. Creates a 1 GB persistent volume for the SQLite database
3. Uploads all secrets to Fly's encrypted store
4. Builds and deploys the container remotely

---

## Set the callback URL

After the first deploy your app is live at `https://<appname>.fly.dev`. Update `callback_url` in your `config.toml`:

```toml
[server]
callback_url = "https://<appname>.fly.dev/pushover/callback"
```

Push the updated config:

```bash
./fly/update.sh
```

Choose **option 1** (Config file). The script re-encodes `config.toml` and redeploys.

---

## Verify

```bash
fly logs --app <appname>
```

Healthy output:

```
[noah] reading: 142 → (no alarm)
[noah] reading: 138 → (no alarm)
```

A new line appears every poll interval. If no alarms are configured to fire at the current BG level, the log will be quiet between readings.

---

## Updating

Run `./fly/update.sh` whenever you need to make changes:

| Option | When to use |
|--------|-------------|
| **1. Config file** | Changed alarm rules, thresholds, or `callback_url` |
| **2. Secrets** | Rotated a Dexcom or Pushover credential |
| **3. Code only** | Pulled new code from git, no config or secret changes |
| **4. Everything** | Changed config and rotated secrets at the same time |

Options 1, 2, and 4 automatically redeploy so changes take effect immediately.

---

## Troubleshooting

**Container won't start**

```bash
fly logs --app <appname>
```

Look for `ERROR: CONFIG_TOML environment variable is required`. Check what secrets are set:

```bash
fly secrets list --app <appname>
```

Re-run `./fly/update.sh` option 1 or 4 to re-upload the config.

**Dexcom auth failures in logs (`session expired` looping)**

Wrong username or password. Run `./fly/update.sh` option 2 to re-enter Dexcom credentials.

**A `${VAR}` reference in config has no matching secret**

The app will fail to connect to Dexcom or Pushover. Run `./fly/update.sh` option 4 to re-enter all values.

**App stops sending readings after a while**

Check `fly logs` for fetch errors. If the Dexcom session expired and re-auth is failing, the credentials may have changed. Use option 2.
