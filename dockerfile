# ---- build stage ----
FROM golang:1.24 AS builder
ENV GOTOOLCHAIN=auto
WORKDIR /app

# 先載入依賴，提高快取命中
COPY go.mod go.sum ./
RUN go mod download

# 複製專案原始碼
COPY . .

# 產生可執行檔（靜態、瘦身）
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o server .

# ---- runtime stage ----
FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /app/server /app/server
ENV TZ=Asia/Taipei
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
