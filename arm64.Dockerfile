FROM arm64v8/alpine:3.18.3

RUN apk update && apk add tzdata \
    && ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

RUN mkdir -p /data

# Copy the Pre-built binary file from the previous stage
COPY webhook.arm64.linux /data/webhook

WORKDIR /data

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
CMD ["./webhook"]
