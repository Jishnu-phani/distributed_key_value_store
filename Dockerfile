FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY g*.go ./

RUN go build -o kvstore .

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/kvstore .

EXPOSE 8080
CMD ["./kvstore"]

# Building the image: docker build -t kvstore:latest .
# running it: docker run -p 8080:8080 kvstore:latest