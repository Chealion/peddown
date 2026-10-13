This is deployed on exe.dev - using lazy, manually repeatable instructions using an [exe.dev](https://exe.dev) VM.

## Initial Deployment

1. Create the VM:

    ```ssh exe.dev new --name=peddown```

2. Build for amd64:

    ```CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build .```

3. Manually copy a variety of files to your VM (optionally be sad this isn't automated):

    ```
    ssh peddown.exe.xyz -- sudo mkdir -p /app && sudo chown -R exedev /app
    scp peddown peddown.exe.xyz:/app/
    scp -r systemd peddown.exe.xyz:/app/
    scp -r creds.env.example peddown.exe.xyz:/app/
    ```

4. SSH to the VM to configure your credentials and set up the systemd service and timer:

    ```
    ssh peddown.exe.xyz
    ```

    Environment Creds:
    ```
    sudo su
    cd /app
    mkdir -p /etc/peddown
    cp creds.env.example /etc/peddown/peddown.env
    vim /etc/peddown/peddown.env
    ```

    Service and Timer:
    ```
    sudo su
    cd /app
    cp -r systemd/peddown* /etc/systemd/service/
    sudo systemctl daemon-reload
    sudo systemctl enable --now peddown.timer peddown.service
    ```

5. Preload the database (if you haven't already). This assumes you're ssh'd into the VM and running as `exedev` *not* root.

    ```
    source /etc/peddown/peddown.env
    ./peddown load-support-data
    ```

6. Check Items:

    ```
    systemctl list-timers peddown.timer
    journalctl -u peddown.service
    ```

## Upgrades/Updates

1. Build
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o peddown .
```

2. Deploy
(Atomic swap in case we have terrible timing)
```bash
scp peddown peddown.exe.xyz:/app/peddown.new
ssh peddown.exe.xyz "mv /app/peddown.new /app/peddown && chmod +x /app/peddown"
```

3. Verify
```bash
ssh peddown.exe.xyz "sudo systemctl start peddown.service && journalctl -u peddown.service -n 30 --no-pager"
```

## Threads credential bootstrapping

Because Threads behaves so differently than the other services, Threads needs a file on disk to store the long-lived (60 day) access token. As such the bootstrapping of this has manual steps.

1. Create a Meta app with the Threads API use case with the `threads_basic` and `threads_content_publish` permissions.
2. Add callbacks to all 3 callback URLs (https://localhost:8000/callback is just fine)
3. Add peddownyyc as a tester to the application and accept the invite
4. Either use the Generate Access Token button on the testing page or get the initial access token by visiting:
    ```
    https://threads.net/oauth/authorize?client_id=<THREADS_APP_ID>&redirect_uri=https://localhost:8000/callback&scope=threads_basic,threads_content_publish&response_type=code
    ```
Since we don't have an app living to receive it grab everything after `code=` and before the last `#` in the URL - that's our access token.
5. Then get the short lived (1 hour) token to interact with the API using curl:
    ```
    curl -X POST "https://graph.threads.net/oauth/access_token" \
      -F client_id="THREADS_APP_ID" \
      -F client_secret="THREADS_APP_SECRET" \
      -F grant_type="authorization_code" \
      -F redirect_uri="https://localhost:8000/callback" \
      -F code="ACCESS_TOKEN_FROM_STEP_4"
    ```
  This returns a JSON object with a user id and the token. The user id should be in your `creds`.
6. We can then exchange the new token for the longer-lived token:
    ```
    curl -s "https://graph.threads.net/access_token?grant_type=th_exchange_token&client_secret=$THREADS_APP_SECRET&access_token=$SHORT_LIVED_TOKEN"
    ```
7. That token should be saved in `/app/threads_token`. (Owned by `exedev` and `0600`)

### Refresh

The token is valid for 60 days and is refreshed automatically whenever the bot
posts, once the file is at least 24 hours old (Meta will not refresh a token
sooner than that). A successful refresh rewrites the file in place and extends it
by another 60 days.

To refresh on demand (for example before a known expiry):

```bash
source /etc/peddown/peddown.env
/app/peddown threads-refresh
```

If a token is rejected, the bot logs `Threads rejected the access token` and sets
the span attribute `exception.slug=threads_token_expired`. That means the token
expired or was revoked (for example the service was down for more than 60 days).
Repeat the steps above to populate a new token.
