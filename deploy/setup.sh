#!/bin/bash
set -e

# CronControl — Production Server Setup
# Run once on the server: bash /opt/croncontrol/app/deploy/setup.sh

echo "==> Setting up CronControl..."

APP_DIR="/opt/croncontrol"
REPO_DIR="$APP_DIR/repo.git"
WORK_DIR="$APP_DIR/app"

# --- Directory structure ---
echo "==> Creating directory structure..."
mkdir -p "$WORK_DIR"
mkdir -p "$REPO_DIR"
mkdir -p "$APP_DIR/data/logs"

# --- Bare git repo ---
echo "==> Initializing bare git repository..."
git init --bare "$REPO_DIR" 2>/dev/null || echo "  Repo already initialized"

# --- Install post-receive hook ---
echo "==> Installing post-receive hook..."
cat > "$REPO_DIR/hooks/post-receive" << 'HOOK'
#!/bin/bash
set -e
TARGET="/opt/croncontrol/app"
GIT_DIR="/opt/croncontrol/repo.git"
BRANCH="main"

while read oldrev newrev ref; do
  if [ "$ref" = "refs/heads/$BRANCH" ]; then
    echo ">>> Deploying CronControl $BRANCH..."
    git --work-tree=$TARGET --git-dir=$GIT_DIR checkout -f $BRANCH

    cd $TARGET

    # Build Go binary
    echo ">>> Building..."
    export CGO_ENABLED=0
    go build -ldflags="-s -w" -o /usr/local/bin/croncontrol .

    # Build frontend and embed
    echo ">>> Building frontend..."
    cd frontend && npm ci --legacy-peer-deps && npm run build && cd ..
    rm -rf internal/frontend/dist && cp -r frontend/dist internal/frontend/dist

    # Rebuild with embedded frontend
    go build -ldflags="-s -w" -o /usr/local/bin/croncontrol .

    # Apply migrations
    echo ">>> Applying migrations..."
    for f in migrations/*.sql; do
      echo "  Applying $f..."
      PGPASSWORD=$CC_DATABASE_PASSWORD psql -U croncontrol -d croncontrol -h localhost -f "$f" 2>&1 | grep -v "already exists" || true
    done

    # Restart service
    echo ">>> Restarting service..."
    systemctl restart croncontrol

    echo ">>> Deploy complete!"
  fi
done
HOOK
chmod +x "$REPO_DIR/hooks/post-receive"

# --- PostgreSQL database ---
echo "==> Creating database..."
sudo -u postgres psql -c "CREATE USER croncontrol WITH PASSWORD 'croncontrol';" 2>/dev/null || echo "  User already exists"
sudo -u postgres psql -c "CREATE DATABASE croncontrol OWNER croncontrol;" 2>/dev/null || echo "  Database already exists"

# --- Install Go (if not present) ---
if ! command -v go &> /dev/null; then
    echo "==> Installing Go..."
    wget -q https://go.dev/dl/go1.24.4.linux-amd64.tar.gz -O /tmp/go.tar.gz
    rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile.d/go.sh
    source /etc/profile.d/go.sh
fi

# --- Install Node.js (if not present) ---
if ! command -v node &> /dev/null; then
    echo "==> Installing Node.js..."
    curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
    apt-get install -y nodejs
fi

# --- Env file ---
if [ ! -f "$APP_DIR/.env" ]; then
    cat > "$APP_DIR/.env" << 'ENV'
CC_SERVER_PORT=8090
CC_DATABASE_HOST=localhost
CC_DATABASE_PORT=5432
CC_DATABASE_USER=croncontrol
CC_DATABASE_PASSWORD=CHANGE_ME_STRONG_PASSWORD
CC_DATABASE_NAME=croncontrol
CC_DATABASE_SSLMODE=disable
CC_AUTH_SESSION_SECRET=CHANGE_ME_RANDOM_SESSION_SECRET
CC_AUTH_ENCRYPTION_KEY=CHANGE_ME_32_BYTES_ENCRYPTION!
CC_SAAS_BASE_URL=https://croncontrol.dev
ENV
    echo "  >>> IMPORTANT: Edit $APP_DIR/.env with real values!"
else
    echo "  .env already exists, skipping"
fi

# --- Systemd service ---
echo "==> Installing systemd service..."
cat > /etc/systemd/system/croncontrol.service << 'SERVICE'
[Unit]
Description=CronControl
After=network.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
ExecStart=/usr/local/bin/croncontrol
WorkingDirectory=/opt/croncontrol/app
EnvironmentFile=/opt/croncontrol/.env
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
SERVICE
systemctl daemon-reload
systemctl enable croncontrol

# --- Nginx ---
echo "==> Installing nginx config..."
cat > /etc/nginx/sites-available/croncontrol << 'NGINX'
server {
    listen 80;
    server_name croncontrol.dev www.croncontrol.dev;
    return 301 https://croncontrol.dev$request_uri;
}

server {
    listen 443 ssl;
    server_name croncontrol.dev www.croncontrol.dev;

    ssl_certificate /etc/letsencrypt/live/croncontrol.dev/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/croncontrol.dev/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    client_max_body_size 10M;

    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
NGINX
ln -sf /etc/nginx/sites-available/croncontrol /etc/nginx/sites-enabled/croncontrol
nginx -t && systemctl reload nginx || echo "  Nginx config test failed — fix manually"

# --- SSL ---
echo "==> Requesting SSL certificate..."
certbot certonly --nginx -d croncontrol.dev -d www.croncontrol.dev --non-interactive --agree-tos || echo "  Certbot failed — run manually"

echo ""
echo "==> Setup complete!"
echo ""
echo "Next steps:"
echo "  1. Edit $APP_DIR/.env with real values"
echo "  2. Add git remote locally:"
echo "     git remote add production root@$(hostname -I | awk '{print $1}'):/opt/croncontrol/repo.git"
echo "  3. Push to deploy:"
echo "     git push production main"
echo "  4. Verify:"
echo "     curl https://croncontrol.dev/health"
