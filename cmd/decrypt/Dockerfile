FROM golang:1.14.4 as builder
WORKDIR /app/
ENV GOBIN /go/bin
COPY *.go ./ 
RUN go get ./...
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o main

FROM alpine:3.12.0
WORKDIR /root/
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/main .
CMD ["./main"]