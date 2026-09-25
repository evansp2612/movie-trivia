FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

FROM alpine:3.20
# tzdata is required for time.LoadLocation (POOL_REFRESH_TZ, e.g. Asia/Jakarta).
RUN apk add --no-cache tzdata ca-certificates
WORKDIR /app
COPY --from=build /out/api /app/api
EXPOSE 8080
CMD ["/app/api"]
