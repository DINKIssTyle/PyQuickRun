#!/bin/bash

APP_NAME="PyQuickBox"
APP_ID="com.dinkisstyle.pyquickbox"
ICON="Icon.png"
PLIST="Info.plist"

echo "Building $APP_NAME..."

# Check if fyne is installed
if ! command -v fyne &> /dev/null; then
    echo "Fyne CLI not found. Installing..."
    export PATH=$PATH:/usr/local/go/bin
    go install fyne.io/fyne/v2/cmd/fyne@latest
    export PATH=$PATH:~/go/bin
fi

# Package the app
~/go/bin/fyne package -os darwin -icon "$ICON" -name "$APP_NAME" -appID "$APP_ID"

# Inject custom Info.plist if it exists
if [ -f "$PLIST" ]; then
    echo "Injecting custom Info.plist..."
    cp "$PLIST" "$APP_NAME.app/Contents/Info.plist"
    echo "Info.plist patched."
fi

echo "Build Complete: $APP_NAME.app"
