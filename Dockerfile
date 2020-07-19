FROM golang:1.13.8 as builder

LABEL maintainer="Luis Pater <webmaster@idotorg.org>"

WORKDIR /app

COPY go.mod go.sum ./

RUN go env -w GOPROXY="goproxy.io"
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o webhook main.go

######## Start a new stage from scratch #######
FROM amd64/alpine:3.10.3

RUN apk update && apk add tzdata \
    && ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

RUN mkdir -p /data

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/webhook /data/webhook

WORKDIR /data

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
CMD ["./webhook"]
