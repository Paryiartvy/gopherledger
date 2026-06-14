FROM golang:1.25 AS builder

WORKDIR /app
COPY go.mod go.sum ./

RUN go mod download

COPY . .
RUN go test -v ./...

RUN CGO_ENABLED=0 GOOS=linux go build -o ./gopherledger ./cmd/server/

FROM gcr.io/distroless/base-debian11

WORKDIR /

COPY --from=builder /app/gopherledger /
COPY --from=builder /app/config.yaml /

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/gopherledger"]