FROM golang:1.17 as builder

WORKDIR /app
COPY . .
RUN go mod download

RUN CGO_ENABLED=0 go build -o /app/main

FROM alpine:latest
COPY --from=builder /app .

EXPOSE 8080
CMD ["/main"]
