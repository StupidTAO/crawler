FROM golang:1.20-alpine as builder
LABEL maintainer="caohaitao_linux@163.com"
WORKDIR /app
COPY . /app
RUN go mod download
RUN go build -o crawler main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/crawler ./
COPY --from=builder /app/config.toml ./
CMD ["./crawler","worker"]