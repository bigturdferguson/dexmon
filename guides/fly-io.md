# Fly.io Deployment — Troubleshooting

For the full setup guide, see [README → Deploy to Fly.io](../README.md#deploy-to-flyio).

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

Wrong Dexcom username or password. Run `./fly/update.sh` option 2 to re-enter Dexcom credentials.

**A `${VAR}` reference in config has no matching secret**

The app will fail to connect to Dexcom or Pushover. Run `./fly/update.sh` option 4 to re-enter all values.

**App stops sending readings after a while**

Check `fly logs` for fetch errors. If the Dexcom session expired and re-auth is failing, the credentials may have changed. Use option 2.
