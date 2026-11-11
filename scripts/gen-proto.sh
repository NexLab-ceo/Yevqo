#!/data/data/com.termux/files/usr/bin/bash
# VELTRIX Dock — генерация Go-кода из proto/veltrix.proto.
# Запускать из корня проекта: bash scripts/gen-proto.sh

set -e

# Проверяем, что мы в корне проекта (там, где лежит go.mod).
if [ ! -f "go.mod" ]; then
    echo "error: go.mod not found. Run from project root."
    exit 1
fi

# Проверяем, что protoc установлен.
if ! command -v protoc >/dev/null 2>&1; then
    echo "error: protoc not found."
    echo "install: pkg install protobuf"
    exit 1
fi

# Проверяем, что protoc-gen-go установлен.
if ! command -v protoc-gen-go >/dev/null 2>&1; then
    echo "error: protoc-gen-go not found."
    echo "install: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
    exit 1
fi

echo "generating Go code from proto/veltrix.proto..."

# Генерируем.
protoc \
    --go_out=. \
    --go_opt=paths=source_relative \
    proto/veltrix.proto

echo "done: proto/veltrix.pb.go"
