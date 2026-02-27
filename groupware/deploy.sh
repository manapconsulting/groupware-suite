#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$SCRIPT_DIR/frontend"
DIST_DIR="$FRONTEND_DIR/dist"
PUBLIC_DIR="$SCRIPT_DIR/public"

echo "=== Groupware Deploy Script ==="

# Build frontend
echo "Building frontend..."
cd "$FRONTEND_DIR"
npm run build

# Create public directory structure
echo "Setting up public directory..."
rm -rf "$PUBLIC_DIR"
mkdir -p "$PUBLIC_DIR/mail/assets"

# Copy files to proper structure
# index.html goes to /mail/
cp "$DIST_DIR/index.html" "$PUBLIC_DIR/mail/"
cp "$DIST_DIR/favicon.svg" "$PUBLIC_DIR/mail/"

# Assets go to /mail/assets/
cp -r "$DIST_DIR/assets/"* "$PUBLIC_DIR/mail/assets/"

# Create root index that redirects to /mail/
cat > "$PUBLIC_DIR/index.html" << 'EOF'
<!DOCTYPE html>
<html>
<head>
    <meta http-equiv="refresh" content="0; url=/mail/">
    <title>Redirecting...</title>
</head>
<body>
    <p>Redirecting to <a href="/mail/">webmail</a>...</p>
</body>
</html>
EOF

echo "Deploy complete!"
echo "Files deployed to: $PUBLIC_DIR"
ls -la "$PUBLIC_DIR"
ls -la "$PUBLIC_DIR/mail"
