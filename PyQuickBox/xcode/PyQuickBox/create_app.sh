#!/bin/bash

APP_NAME="PyQuickBox"
BUILD_DIR=".build/release"
APP_BUNDLE="${APP_NAME}.app"
OUTPUT_DIR="xcode/PyQuickBox/Dist"

# Ensure we are in the package root
cd "$(dirname "$0")"

echo "Building Release..."
swift build -c release

if [ $? -ne 0 ]; then
    echo "Build failed."
    exit 1
fi

echo "Creating App Bundle..."
mkdir -p "$APP_BUNDLE/Contents/MacOS"
mkdir -p "$APP_BUNDLE/Contents/Resources"

# Copy Binary
cp ".build/release/$APP_NAME" "$APP_BUNDLE/Contents/MacOS/"

# Create Info.plist
cat > "$APP_BUNDLE/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>${APP_NAME}</string>
    <key>CFBundleIdentifier</key>
    <string>com.dinkisstyle.pyquickbox</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0</string>
    <key>CFBundleVersion</key>
    <string>1</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSMinimumSystemVersion</key>
    <string>13.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF

echo "App Bundle created at $(pwd)/$APP_BUNDLE"
echo "You can move this to your /Applications folder."
open -R "$APP_BUNDLE"
