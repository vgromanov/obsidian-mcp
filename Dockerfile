# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/obsidian-mcp ./cmd/obsidian-mcp

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /out/obsidian-mcp /usr/local/bin/obsidian-mcp
ENTRYPOINT ["/usr/local/bin/obsidian-mcp"]
