# Simple cross-compilation Makefile
# Requires Go 1.20+ and a properly configured environment.

BINARY_NAME = windows_exporter
PKG         = ./cmd/windowsexporter

# Default build (for current OS/ARCH)
build:
	go build -o bin/$(BINARY_NAME) $(PKG)

# Windows: x86 (386) and x64 (amd64)
build-windows:
	GOOS=windows GOARCH=386   go build -o bin/windows_386/$(BINARY_NAME).exe   $(PKG)
	GOOS=windows GOARCH=amd64 go build -o bin/windows_amd64/$(BINARY_NAME).exe $(PKG)

# Linux: x86 (386), x64 (amd64), ARM
build-linux:
	GOOS=linux GOARCH=386    go build -o bin/linux_386/$(BINARY_NAME)   $(PKG)
	GOOS=linux GOARCH=amd64  go build -o bin/linux_amd64/$(BINARY_NAME) $(PKG)
	GOOS=linux GOARCH=arm    go build -o bin/linux_arm/$(BINARY_NAME)   $(PKG)

# macOS: x64 (amd64) and ARM (arm64)
build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o bin/darwin_amd64/$(BINARY_NAME) $(PKG)
	GOOS=darwin GOARCH=arm64 go build -o bin/darwin_arm64/$(BINARY_NAME) $(PKG)

# Android: ARM
build-android:
	GOOS=android GOARCH=arm  go build -o bin/android_arm/$(BINARY_NAME) $(PKG)

# Build all
build-all: build-windows build-linux build-darwin build-android

clean:
	rm -rf bin
