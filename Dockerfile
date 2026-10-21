# ---- build stage ----
FROM golang:1.26-alpine AS builder
WORKDIR /src

# 先只拷贝依赖描述，利用 Docker 层缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/seckill .

# ---- run stage ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /out/seckill /app/seckill
EXPOSE 8080
ENTRYPOINT ["/app/seckill"]
