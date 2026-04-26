#!/bin/bash
# 1. 以前のバイナリとZIPを削除
rm -f bootstrap main.zip

# 2. AWS Lambda (Amazon Linux 2) 用にクロスコンパイル
# -tags lambda.norpc はバイナリサイズを軽量化するためのオプションです
GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap main.go

# 3. ZIP圧縮（Lambdaはこの形式を要求します）
zip main.zip bootstrap

echo "Build complete: backend/main.zip"