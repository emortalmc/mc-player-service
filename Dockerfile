FROM golang:alpine as go
WORKDIR /app
ENV GO111MODULE=on

COPY go.mod .
RUN go mod download

COPY . .
RUN go build -o mc-player-service ./cmd

FROM alpine

COPY --from=go /app/mc-player-service /app/mc-player-service
CMD ["/app/mc-player-service"]