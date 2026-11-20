FROM golang:1.26.7-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath \
    -o /out/server ./cmd/server

RUN CGO_ENABLED=0 go build -trimpath \
    -o /out/telegram ./telegram

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/server ./server
COPY --from=builder /out/telegram ./telegram

EXPOSE 8080

ENTRYPOINT ["./server"]
