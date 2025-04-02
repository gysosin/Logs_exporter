# Simple cross-compilation Makefile
# Requires Go 1.20+ and a properly configured environment.

BINARY_NAME = windows_exporter
PKG         = ./cmd/windowsexporter

ifeq ($(OS),Windows_NT)
	BUILD_WINDOWS_386   = set GOOS=windows&& set GOARCH=386&& go build -o bin/windows_386/$(BINARY_NAME).exe $(PKG)
	BUILD_WINDOWS_AMD64 = set GOOS=windows&& set GOARCH=amd64&& go build -o bin/windows_amd64/$(BINARY_NAME).exe $(PKG)
	BUILD_LINUX_386     = set GOOS=linux&& set GOARCH=386&& go build -o bin/linux_386/$(BINARY_NAME) $(PKG)
	BUILD_LINUX_AMD64   = set GOOS=linux&& set GOARCH=amd64&& go build -o bin/linux_amd64/$(BINARY_NAME) $(PKG)
	BUILD_LINUX_ARM     = set GOOS=linux&& set GOARCH=arm&& go build -o bin/linux_arm/$(BINARY_NAME) $(PKG)
	BUILD_DARWIN_AMD64  = set GOOS=darwin&& set GOARCH=amd64&& go build -o bin/darwin_amd64/$(BINARY_NAME) $(PKG)
	BUILD_DARWIN_ARM64  = set GOOS=darwin&& set GOARCH=arm64&& go build -o bin/darwin_arm64/$(BINARY_NAME) $(PKG)
	BUILD_ANDROID_ARM   = set GOOS=android&& set GOARCH=arm&& go build -o bin/android_arm/$(BINARY_NAME) $(PKG)
else
	BUILD_WINDOWS_386   = GOOS=windows GOARCH=386 go build -o bin/windows_386/$(BINARY_NAME).exe $(PKG)
	BUILD_WINDOWS_AMD64 = GOOS=windows GOARCH=amd64 go build -o bin/windows_amd64/$(BINARY_NAME).exe $(PKG)
	BUILD_LINUX_386     = GOOS=linux GOARCH=386 go build -o bin/linux_386/$(BINARY_NAME) $(PKG)
	BUILD_LINUX_AMD64   = GOOS=linux GOARCH=amd64 go build -o bin/linux_amd64/$(BINARY_NAME) $(PKG)
	BUILD_LINUX_ARM     = GOOS=linux GOARCH=arm go build -o bin/linux_arm/$(BINARY_NAME) $(PKG)
	BUILD_DARWIN_AMD64  = GOOS=darwin GOARCH=amd64 go build -o bin/darwin_amd64/$(BINARY_NAME) $(PKG)
	BUILD_DARWIN_ARM64  = GOOS=darwin GOARCH=arm64 go build -o bin/darwin_arm64/$(BINARY_NAME) $(PKG)
	BUILD_ANDROID_ARM   = GOOS=android GOARCH=arm go build -o bin/android_arm/$(BINARY_NAME) $(PKG)
endif

# Default build (for current OS/ARCH)
build:
	go build -o bin/$(BINARY_NAME) $(PKG)

# Windows: x86 (386) and x64 (amd64)
build-windows:
	$(BUILD_WINDOWS_386)
	$(BUILD_WINDOWS_AMD64)

# Linux: x86 (386), x64 (amd64), ARM
build-linux:
	$(BUILD_LINUX_386)
	$(BUILD_LINUX_AMD64)
	$(BUILD_LINUX_ARM)

# macOS: x64 (amd64) and ARM (arm64)
build-darwin:
	$(BUILD_DARWIN_AMD64)
	$(BUILD_DARWIN_ARM64)

# Android: ARM
build-android:
	$(BUILD_ANDROID_ARM)

# Build all
build-all: build-windows build-linux build-darwin build-android

clean:
	rm -rf bin
