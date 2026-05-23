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

If you do not have a domain, Cloudflare's `trycloudflare.com` offers free subdomains — run the tunnel once to get your URL:

```bash
cloudflared tunnel run dexmon
# Your tunnel hostname appears in the startup output
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
