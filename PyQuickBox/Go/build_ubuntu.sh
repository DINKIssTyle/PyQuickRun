#!/bin/bash

APP_NAME="PyQuickBox"
APP_ID="com.dinkisstyle.pyquickbox"
ICON="Icon.png"

echo "----------------------------------------"
echo " PyQuickBox Ubuntu Builder"
echo "----------------------------------------"

# 1. Check for Go
if ! command -v go &> /dev/null; then
    echo "[ERROR] Go is not installed."
    echo "Please install Go: https://go.dev/doc/install"
    exit 1
fi

# 2. Check for GCC
if ! command -v gcc &> /dev/null; then
    echo "[ERROR] GCC is not installed."
    echo "Please run: sudo apt-get install build-essential"
    exit 1
fi

# 3. Check for Fyne tool
if ! command -v fyne &> /dev/null; then
    echo "[INFO] Installing Fyne toolkit..."
    # Ensure Go bin is in path for this session
    export PATH=$PATH:$(go env GOPATH)/bin
    go install fyne.io/fyne/v2/cmd/fyne@latest
fi

echo "[INFO] Tidying dependencies..."
go mod tidy

echo "[INFO] Building package..."
echo "Note: If this fails, ensure you have graphics dependencies installed:"
echo "sudo apt-get install libgl1-mesa-dev xorg-dev"
echo ""

# Use Fyne package to create the .tar.xz release structure for Linux
fyne package -os linux -icon "$ICON" -name "$APP_NAME" -appID "$APP_ID"

if [ $? -eq 0 ]; then
    echo ""
    echo "----------------------------------------"
    echo " [SUCCESS] Build Complete: $APP_NAME.tar.xz"
    echo "----------------------------------------"
    echo "To install/run:"
    echo "1. Extract the tarball: tar -xf $APP_NAME.tar.xz"
    echo "2. Run: ./usr/local/bin/$APP_NAME"
else
    echo ""
    echo " [FAILURE] Build Failed."
fi
