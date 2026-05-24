# Deployment Guides Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three step-by-step deployment guides in `guides/` and update the README to be a scannable reference doc that links to them.

**Architecture:** Four documentation tasks, each self-contained. The README gains a "How it works" section, an accounts-vs-recipients paragraph, and a Deployment section that is a link table. Three new files in `guides/` cover Raspberry Pi, Fly.io, and GitHub Actions CI/CD in full step-by-step detail for a zero-knowledge reader.

**Tech Stack:** Markdown, Git

---

## File Structure

```
guides/
  raspberry-pi.md     ← new: full Pi deployment walkthrough
  fly-io.md           ← new: full Fly.io walkthrough
  github-actions.md   ← new: GitHub Actions CI/CD setup
README.md             ← modified: three targeted additions/replacements
```

---

### Task 1: Update README.md

**Files:**
- Modify: `README.md`

**Context:** The README needs three targeted changes. Do not rewrite anything else.

1. Add a "How It Works" section between the intro paragraph and `## Features`.
2. Add an accounts-vs-recipients paragraph in the `## Configuration` section, right before the `### [server]` subsection.
3. Replace the entire `## Deployment` section (from `### Raspberry Pi (systemd)` through the end of the `### Fly.io` block, including the trailing `---`) with a short link table.

- [ ] **Step 1: Add "How It Works" section**

Find this text in README.md:
```
---

## Features
```

Replace with:
```
---

## How It Works

dexmon runs a polling loop for each configured Dexcom account. Every `poll_interval`, it fetches the latest CGM reading from the Dexcom Share API. Each reading is evaluated against the account's alarm rules — an alarm fires when the blood glucose value crosses a threshold **and** the current trend matches the configured filter. Fired alarms are dispatched as Pushover notifications at the configured priority. Emergency-priority alarms retry until acknowledged; when a recipient acknowledges from the Pushover app, Pushover POSTs to dexmon's callback URL, which stops retrying and optionally snoozes for that recipient.

---

## Features
```

- [ ] **Step 2: Add accounts-vs-recipients paragraph**

Find this text in README.md:
```
All configuration lives in a single TOML file.
```

Replace with:
```
All configuration lives in a single TOML file.

**Accounts** are the Dexcom Share logins dexmon monitors — one entry per patient. **Recipients** are the people who receive Pushover notifications. They are separate: multiple recipients can watch the same account, and each has independent snooze and backoff state. A recipient who silences an alarm does not affect what others receive.
```

- [ ] **Step 3: Replace the Deployment section**

Find this block (from the `## Deployment` heading through the end of the Fly.io section, ending just before `## Flags`):
```
## Deployment

### Raspberry Pi (systemd)

1. Build for ARM:

   ```bash
   GOOS=linux GOARCH=arm64 go build -o dexmon .
   ```

   Or on the Pi itself: `go build -o dexmon .`

2. Copy files to the host:

   ```bash
   scp dexmon config.toml pi@yourpi:/opt/dexmon/
   ```

3. Create a secrets file (mode 600, root-only):

   ```bash
   sudo tee /opt/dexmon/secrets.env > /dev/null <<EOF
   PUSHOVER_APP_TOKEN=...
   PUSHOVER_USER_KEY_BRANDON=...
   PUSHOVER_USER_KEY_SARAH=...
   DEXCOM_USER_JESSICA=...
   DEXCOM_PASS_JESSICA=...
   HEALTHCHECKS_PING_URL=...
   EOF
   sudo chmod 600 /opt/dexmon/secrets.env
   sudo chown root:root /opt/dexmon/secrets.env
   ```

4. Create a system user:

   ```bash
   sudo useradd --system --no-create-home dexmon
   sudo chown dexmon:dexmon /opt/dexmon
   ```

5. Install the systemd unit:

   ```bash
   sudo cp dexmon.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable --now dexmon
   ```

6. Check logs:

   ```bash
   journalctl -u dexmon -f
   ```

### Pushover Callback (for emergency acknowledgment)

Emergency alarms support acknowledgment and snooze via Pushover callback. Pushover POSTs to `callback_url` when a recipient acknowledges.

To receive callbacks, `callback_url` must be reachable from the internet. Options:
- **Port forward** port 8080 on your router to the Pi
- **Reverse proxy** with nginx or Caddy on the Pi
- **Cloudflare Tunnel** or similar (no open port required)

If you don't configure a callback URL, emergency alarms still fire and retry — they just can't be acknowledged from the Pushover app.

### Fly.io

dexmon ships with scripts for automated Fly.io deployment. The setup script guides first-time Fly.io users from zero to a running instance.

#### Prerequisites

Install `flyctl`:

```bash
curl -L https://fly.io/install.sh | sh
```

You'll also need a `config.toml` based on `config.toml.example` with your alarm rules filled in. Secrets (`${VAR}` values) are prompted for by the deploy script — do not put real credentials in the file.

#### Initial deploy

```bash
./fly/deploy.sh
```

The script:
1. Checks for `flyctl` and walks through login if needed
2. Prompts for an app name (globally unique on Fly.io) and region
3. Creates the Fly app and a 1 GB persistent volume for the SQLite database
4. Scans your `config.toml` for `${VAR}` references and prompts for each value
5. Uploads all secrets (including the encoded config) to Fly's encrypted secret store
6. Deploys the container

After the first deploy, update `callback_url` in your `config.toml`:

```toml
[server]
callback_url = "https://<your-app-name>.fly.dev/pushover/callback"
```

Then push the updated config:

```bash
./fly/update.sh   # choose option 1 or 4
```

#### Updating

```bash
./fly/update.sh
```

| Option | What it does |
|---|---|
| 1. Config file | Re-encode `config.toml`, push as secret, redeploy |
| 2. Secrets | Re-prompt for `${VAR}` values, push secrets, redeploy |
| 3. Code only | `fly deploy` — no secret changes |
| 4. Everything | Config + secrets + redeploy |

#### Optional: GitHub Actions CI/CD

To automatically deploy on every push to `main`:

1. Create a deploy token:

   ```bash
   fly tokens create deploy -x 999999h
   ```

2. Add it to your GitHub repo: **Settings → Secrets and variables → Actions → New repository secret**
   - Name: `FLY_API_TOKEN`
   - Value: the token from step 1

The workflow file is already at `.github/workflows/fly-deploy.yml`. Before enabling CI/CD, make sure you have run `./fly/deploy.sh` at least once and committed the updated `fly/fly.toml` — it must contain your real app name, not the placeholder. To disable CI/CD, delete the workflow file or remove the `FLY_API_TOKEN` secret.

---
```

Replace with:
```
## Deployment

| Platform | Guide |
|---|---|
| Raspberry Pi (home, always-on) | [guides/raspberry-pi.md](guides/raspberry-pi.md) |
| Fly.io (cloud, no hardware needed) | [guides/fly-io.md](guides/fly-io.md) |
| GitHub Actions CI/CD (auto-deploy on push) | [guides/github-actions.md](guides/github-actions.md) |

---
```

- [ ] **Step 4: Verify the README changes**

```bash
grep -n "How It Works\|Accounts.*recipients\|guides/raspberry-pi\|guides/fly-io\|guides/github-actions" README.md
```

Expected: at least one match for each of those five strings.

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "docs: add How It Works section, accounts vs recipients, replace Deployment section with guide links"
```

---

### Task 2: guides/raspberry-pi.md

**Files:**
- Create: `guides/raspberry-pi.md`

**Context:** Full step-by-step guide for a zero-knowledge reader. The `dexmon.service` file already exists in the repo at `dexmon.service` — it runs as user `dexmon`, working directory `/opt/dexmon`, reads secrets from `/opt/dexmon/secrets.env`. Go version in this project is 1.26.3. Cloudflare Tunnel is the recommended callback solution because it requires no port forwarding and no static IP.

- [ ] **Step 1: Create `guides/raspberry-pi.md`**

Create the file with this exact content:

````markdown
# Deploying dexmon on a Raspberry Pi

This guide walks through installing dexmon as a persistent background service on a Raspberry Pi running Raspberry Pi OS Lite 64-bit (Bookworm). By the end, dexmon starts automatically on boot, runs under a dedicated system user, and can receive emergency alarm acknowledgments via Cloudflare Tunnel.

---

## Prerequisites

- Raspberry Pi with **Raspberry Pi OS Lite 64-bit (Bookworm)** installed and SSH access confirmed
- A **Pushover** account at pushover.net with an application created (free)
- **Dexcom Share** enabled on the patient's Dexcom G-series app (Settings → Share → Invite Followers)

---

## Get your credentials

Gather these before starting.

**Pushover app token**
Go to [pushover.net](https://pushover.net) → scroll to **Your Applications** → click your app → copy the **API Token/Key**. This is the value for `PUSHOVER_APP_TOKEN`.

**Pushover user key**
On [pushover.net](https://pushover.net), your **User Key** is shown at the top of the page after logging in. Each person who receives notifications needs their own user key.

**Dexcom Share credentials**
The email address and password used to log in to the Dexcom mobile app. Share must be enabled — if the app shows a Sharing or Followers section, it's on.

---

## Install Go on the Pi

SSH into the Pi, then:

```bash
# Download Go 1.26.3 for ARM64
wget https://go.dev/dl/go1.26.3.linux-arm64.tar.gz

# Extract to /usr/local
sudo tar -C /usr/local -xzf go1.26.3.linux-arm64.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
source ~/.profile

# Verify
go version
# Expected: go version go1.26.3 linux/arm64
```

---

## Get dexmon

```bash
git clone https://github.com/bigturdferguson/dexmon.git
cd dexmon
go build -o dexmon .
```

**Alternative: cross-compile on your dev machine**

If you prefer not to install Go on the Pi, build on your Mac or Linux machine and copy the binary:

```bash
# On your dev machine, from the dexmon repo:
GOOS=linux GOARCH=arm64 go build -o dexmon .
scp dexmon pi@<pi-ip>:/tmp/dexmon
```

---

## Set up the directory and config

**1. Create the dexmon directory:**

```bash
sudo mkdir -p /opt/dexmon
```

**2. Copy the binary:**

```bash
# If built on the Pi:
sudo cp ~/dexmon/dexmon /opt/dexmon/dexmon

# If copied from dev machine:
sudo cp /tmp/dexmon /opt/dexmon/dexmon

sudo chmod +x /opt/dexmon/dexmon
```

**3. Copy and edit the config:**

```bash
sudo cp ~/dexmon/config.toml.example /opt/dexmon/config.toml
sudo nano /opt/dexmon/config.toml
```

Fill in your alarm rules and recipients. Leave all `${VAR}` placeholders as-is — they are expanded from `secrets.env` at runtime. Set `callback_url` to empty for now (you will fill it in after setting up Cloudflare Tunnel):

```toml
[server]
callback_port = 8080
callback_url  = ""
```

**4. Create the secrets file:**

```bash
sudo nano /opt/dexmon/secrets.env
```

Add one line per credential (no quotes, no spaces around `=`):

```
PUSHOVER_APP_TOKEN=your_app_token_here
PUSHOVER_USER_KEY_BRANDON=your_user_key_here
DEXCOM_USER_NOAH=noah@example.com
DEXCOM_PASS_NOAH=dexcom_password_here
```

Match the variable names to whatever `${VAR}` references appear in your `config.toml`.

**5. Lock down the secrets file:**

```bash
sudo chmod 600 /opt/dexmon/secrets.env
sudo chown root:root /opt/dexmon/secrets.env
```

---

## Create a system user

Running dexmon as a dedicated system user limits what the process can access:

```bash
sudo useradd --system --no-create-home dexmon
sudo chown -R dexmon:dexmon /opt/dexmon
# secrets.env stays root-owned but dexmon-readable via the service EnvironmentFile directive
sudo chmod 640 /opt/dexmon/secrets.env
sudo chown root:dexmon /opt/dexmon/secrets.env
```

---

## Install as a systemd service

**1. Copy the service file from the repo:**

```bash
sudo cp ~/dexmon/dexmon.service /etc/systemd/system/dexmon.service
```

The service file configures dexmon to:
- Run as the `dexmon` user
- Load secrets from `/opt/dexmon/secrets.env`
- Restart automatically if it crashes (10-second delay)
- Start after the network is available

**2. Enable and start:**

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now dexmon
```

**3. Verify:**

```bash
sudo systemctl status dexmon
```

Expected: `Active: active (running)`

**4. Watch the logs:**

```bash
journalctl -u dexmon -f
```

Healthy output looks like:

```
[noah] reading: 142 → (no alarm)
[noah] reading: 138 → (no alarm)
```

A new line appears every poll interval (default: 5 minutes).

---

## Enable emergency callbacks via Cloudflare Tunnel

Emergency alarms retry until acknowledged via Pushover. For acknowledgment to work, dexmon's webhook server (port 8080) must be reachable from the internet. Cloudflare Tunnel provides a stable public HTTPS URL without opening any ports on your router.

**1. Install cloudflared:**

```bash
curl -L --output cloudflared.deb \
  https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64.deb
sudo dpkg -i cloudflared.deb
cloudflared --version
```

**2. Log in to Cloudflare** (free account at dash.cloudflare.com):

```bash
cloudflared tunnel login
```

On a headless Pi, this prints a URL — open it in a browser on another machine and authorize.

**3. Create a named tunnel:**

```bash
cloudflared tunnel create dexmon
```

Note the tunnel ID (a UUID like `a1b2c3d4-1234-...`) printed in the output.

**4. Configure the tunnel:**

```bash
mkdir -p ~/.cloudflared
```

Create `~/.cloudflared/config.yml` with this content (substitute your tunnel ID):

```yaml
tunnel: <your-tunnel-id>
credentials-file: /home/pi/.cloudflared/<your-tunnel-id>.json

ingress:
  - service: http://localhost:8080
```

**5. Route a hostname to the tunnel:**

If you have a domain managed by Cloudflare:

```bash
cloudflared tunnel route dns dexmon dexmon.yourdomain.com
```

If you do not have a domain, Cloudflare's `trycloudflare.com` offers free stable subdomains:

```bash
cloudflared tunnel run dexmon
# Look for a line like: INF +----------------------------+
# Your tunnel URL will appear in the output
```

**6. Update `callback_url` in config.toml:**

```bash
sudo nano /opt/dexmon/config.toml
```

```toml
[server]
callback_url = "https://dexmon.yourdomain.com/pushover/callback"
```

**7. Install cloudflared as a persistent service:**

```bash
sudo cloudflared service install
sudo systemctl enable --now cloudflared
```

**8. Restart dexmon to pick up the new callback URL:**

```bash
sudo systemctl restart dexmon
```

---

## Verify

Confirm dexmon is receiving readings:

```bash
journalctl -u dexmon -f
```

You should see a reading logged every 5 minutes. If BG is in range and no alarms are configured to fire, you will see quiet log lines with no alarm dispatch.

To test an alarm: temporarily set a threshold that the current BG value would cross, wait one poll interval, and check for a Pushover notification.

---

## Troubleshooting

**`session expired` repeating in logs**
Wrong Dexcom username or password. Edit `/opt/dexmon/secrets.env` and restart:
```bash
sudo nano /opt/dexmon/secrets.env
sudo systemctl restart dexmon
```

**Pushover notifications not arriving**
- Confirm `PUSHOVER_APP_TOKEN` is the app's API Token, not a user key
- Confirm `PUSHOVER_USER_KEY_<NAME>` is the recipient's User Key from their Pushover profile page
- Check logs for `dispatch error` entries

**Service fails to start**
```bash
journalctl -u dexmon -n 50
```
Common causes: syntax error in `config.toml`, missing variable in `secrets.env`, binary not executable.

**Emergency alarms cannot be acknowledged**
Cloudflared may not be running:
```bash
sudo systemctl status cloudflared
```
Also verify the URL in `callback_url` ends with `/pushover/callback` and matches the hostname configured in `~/.cloudflared/config.yml`.
````

- [ ] **Step 2: Verify the file**

```bash
grep -c "##" guides/raspberry-pi.md
```

Expected: 9 or more (one per section heading).

```bash
grep -n "Prerequisites\|credentials\|Install Go\|systemd\|Cloudflare\|Troubleshoot" guides/raspberry-pi.md
```

Expected: each of those words appears at least once.

- [ ] **Step 3: Commit**

```bash
git add guides/raspberry-pi.md
git commit -m "docs: add Raspberry Pi deployment guide"
```

---

### Task 3: guides/fly-io.md

**Files:**
- Create: `guides/fly-io.md`

**Context:** Full step-by-step Fly.io guide for a reader with no prior Fly.io experience. Key facts: the deploy script is at `fly/deploy.sh`, the update script at `fly/update.sh`. The Fly.io app provides an HTTPS URL at `https://<appname>.fly.dev` which becomes the `callback_url`. Secrets are stored in Fly's encrypted secret store and never touch the filesystem. The update menu has 4 options (config / secrets / code / everything).

- [ ] **Step 1: Create `guides/fly-io.md`**

Create the file with this exact content:

````markdown
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
````

- [ ] **Step 2: Verify the file**

```bash
grep -n "Prerequisites\|credentials\|deploy.sh\|callback_url\|Updating\|Troubleshoot" guides/fly-io.md
```

Expected: each word appears at least once.

- [ ] **Step 3: Commit**

```bash
git add guides/fly-io.md
git commit -m "docs: add Fly.io deployment guide"
```

---

### Task 4: guides/github-actions.md

**Files:**
- Create: `guides/github-actions.md`

**Context:** Guide for setting up the optional CI/CD workflow. The workflow file already exists at `.github/workflows/fly-deploy.yml`. Key facts: it triggers on push to `main`, uses `--remote-only` so no Docker needed in CI, requires `FLY_API_TOKEN` GitHub secret. The user must have completed the Fly.io guide first and must have their real app name in `fly/fly.toml` (not the `DEXMON_APP_NAME` placeholder).

- [ ] **Step 1: Create `guides/github-actions.md`**

Create the file with this exact content:

````markdown
# GitHub Actions CI/CD for Fly.io

This guide sets up automatic deployment to Fly.io on every push to `main`. After setup, pushing code changes triggers a Fly.io rebuild and redeploy automatically.

---

## Prerequisites

- The [Fly.io deployment guide](fly-io.md) is complete — dexmon is running on Fly.io
- `fly/fly.toml` in your repo has your **real app name** committed (not the placeholder)
- The repo is hosted on GitHub

Verify `fly/fly.toml` is correct:

```bash
grep "^app" fly/fly.toml
# Expected: app = 'your-real-app-name'
# Not: app = 'DEXMON_APP_NAME'
```

If it shows the placeholder, run `./fly/deploy.sh` first and commit the result.

---

## Create a deploy token

A deploy token lets GitHub Actions deploy to your app without exposing your full Fly.io account credentials.

```bash
fly tokens create deploy -x 999999h
```

Copy the output — it is only shown once. The `-x 999999h` flag sets an expiry of roughly 114 years. Shorten this if you prefer to rotate tokens more frequently.

---

## Add the token to GitHub

1. Open your repository on GitHub
2. Go to **Settings** → **Secrets and variables** → **Actions**
3. Click **New repository secret**
4. **Name:** `FLY_API_TOKEN`
5. **Value:** paste the token from the previous step
6. Click **Add secret**

---

## How it works

The workflow file at `.github/workflows/fly-deploy.yml` runs on every push to `main`:

1. Checks out the repository
2. Installs `flyctl`
3. Runs `flyctl deploy --remote-only --config fly/fly.toml`

**`--remote-only`** tells Fly.io to build the Docker image on their own infrastructure. The GitHub Actions runner does not need Docker installed, and you do not need to push a local image. Builds are fast and free on Fly's side.

**`concurrency: deploy-group`** ensures only one deploy runs at a time. If you push twice in quick succession, the second deploy queues behind the first.

The workflow deploys code changes only. It does not modify Fly secrets. To update config or credentials, use `./fly/update.sh` locally.

---

## Trigger your first automated deploy

Push any commit to `main`:

```bash
git add .
git commit -m "chore: trigger first CI deploy"
git push origin main
```

Watch the deployment:

1. Go to your repo on GitHub
2. Click the **Actions** tab
3. Click **Deploy to Fly.io** → click the in-progress run

Each step executes in real time. The full deploy takes 1-3 minutes.

---

## Verify

A green checkmark in the Actions tab means the deploy succeeded. Confirm the new version is live:

```bash
fly logs --app <appname>
```

The timestamps on recent log lines should match the time of your push.

---

## Disabling CI/CD

**To pause without deleting the workflow:**
Go to GitHub → Settings → Secrets and variables → Actions → find `FLY_API_TOKEN` → delete it. The workflow will fail immediately with an auth error on the next push, effectively disabling deploys. Re-add the secret to re-enable.

**To remove permanently:**
Delete `.github/workflows/fly-deploy.yml` from the repo and push.

---

## Troubleshooting

**Workflow fails immediately with "Error: No FLY_API_TOKEN set"**
The secret is missing or misnamed. Go to Settings → Secrets and verify the name is exactly `FLY_API_TOKEN` (all caps, underscores).

**"App not found" error during deploy**
`fly/fly.toml` still has the `DEXMON_APP_NAME` placeholder. Run `./fly/deploy.sh` locally, then commit and push the updated `fly/fly.toml`.

**Deploy succeeds in CI but app doesn't start**
Check `fly logs --app <appname>` for startup errors. Usually a missing secret — use `./fly/update.sh` locally to set it.

**Workflow runs on the wrong branch**
The workflow triggers on pushes to `main`. If your default branch is named differently, edit `.github/workflows/fly-deploy.yml` and change `main` to your branch name.
````

- [ ] **Step 2: Verify the file**

```bash
grep -n "Prerequisites\|deploy token\|FLY_API_TOKEN\|remote-only\|Troubleshoot" guides/github-actions.md
```

Expected: each phrase appears at least once.

- [ ] **Step 3: Commit**

```bash
git add guides/github-actions.md
git commit -m "docs: add GitHub Actions CI/CD setup guide"
```

---

## Self-Review

**Spec coverage:**

| Spec requirement | Task |
|---|---|
| README: "How it works" section | Task 1 |
| README: accounts vs recipients | Task 1 |
| README: Deployment section replaced with link table | Task 1 |
| guides/raspberry-pi.md: Prerequisites | Task 2 |
| guides/raspberry-pi.md: Get credentials | Task 2 |
| guides/raspberry-pi.md: Install Go | Task 2 |
| guides/raspberry-pi.md: Build dexmon | Task 2 |
| guides/raspberry-pi.md: Configure + secrets.env | Task 2 |
| guides/raspberry-pi.md: systemd service | Task 2 |
| guides/raspberry-pi.md: Cloudflare Tunnel | Task 2 |
| guides/raspberry-pi.md: Verify | Task 2 |
| guides/raspberry-pi.md: Troubleshooting | Task 2 |
| guides/fly-io.md: Prerequisites | Task 3 |
| guides/fly-io.md: Get credentials | Task 3 |
| guides/fly-io.md: Prepare config.toml | Task 3 |
| guides/fly-io.md: deploy.sh walkthrough | Task 3 |
| guides/fly-io.md: Set callback URL | Task 3 |
| guides/fly-io.md: Verify | Task 3 |
| guides/fly-io.md: Updating menu | Task 3 |
| guides/fly-io.md: Troubleshooting | Task 3 |
| guides/github-actions.md: Prerequisites | Task 4 |
| guides/github-actions.md: Create deploy token | Task 4 |
| guides/github-actions.md: Add to GitHub | Task 4 |
| guides/github-actions.md: How it works | Task 4 |
| guides/github-actions.md: Trigger first deploy | Task 4 |
| guides/github-actions.md: Verify | Task 4 |
| guides/github-actions.md: Disabling CI/CD | Task 4 |
| guides/github-actions.md: Troubleshooting | Task 4 |

All spec sections covered. No placeholders. Content is consistent across tasks.
