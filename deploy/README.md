This is deployed on exe.dev - using lazy, manually repeatable instructions using an [exe.dev](https://exe.dev) VM.

## Initial Deployment

1. Create the VM:

    ```ssh exe.dev new --name=peddown```

2. Build for amd64:

    ```GOOS=linux GOARCH=amd64 go build .```

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
