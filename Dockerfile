FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/batys-monitor ./cmd/main.go


FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata python3 py3-pip \
    && pip3 install --break-system-packages python-docx

COPY --from=builder /app/batys-monitor .
COPY --from=builder /app/scripts ./scripts

CMD ["sh", "-c", "SERVER_PORT=${PORT:-3000} ./batys-monitor"]