#!/usr/bin/env zsh
# deploy_android.sh — Build and install the Sudoku app on a connected Android device.
# Enable USB debugging on the device: Settings → Developer options → USB debugging.

set -e

export JAVA_HOME="/Applications/Android Studio.app/Contents/jbr/Contents/Home"
export ANDROID_NDK_HOME="$ANDROID_HOME/ndk/27.2.12479018"
export PATH="$JAVA_HOME/bin:$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/build-tools/35.0.0:$HOME/go/bin:$PATH"

echo "==> Building Android APK..."
fyne package --target android --app-id com.matthewfetzer.sudoku

echo "==> Installing on Android device..."
adb shell settings put global verifier_verify_adb_installs 0
adb install -r Sudoku.apk

echo ""
echo "✅ Done! Find 'Sudoku' on your Android device."
