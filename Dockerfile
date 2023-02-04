FROM golang:alpine as go
WORKDIR /app
ENV GO111MODULE=on

COPY go.mod .
RUN go mod download

COPY . .
RUN go build -o mc-player-service ./cmd

FROM alpine

WORKDIR /app

COPY --from=go /app/mc-player-service ./mc-player-service
COPY run/config.yaml ./config.yaml
CMD ["./mc-player-service"]