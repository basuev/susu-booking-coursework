FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/booking-svc ./cmd/booking-svc

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/booking-svc /bin/booking-svc

EXPOSE 50051

ENTRYPOINT ["/bin/booking-svc"]
