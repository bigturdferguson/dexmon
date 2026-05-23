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
