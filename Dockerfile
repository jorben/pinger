FROM golang:1.20-alpine as build
WORKDIR /app
COPY ["go.mod", "go.sum", "main.go", "./"]
RUN go mod tidy && go build

FROM alpine
# This library attempts to send an "unprivileged" ping via UDP. On Linux, this must be enabled
RUN sysctl -w net.ipv4.ping_group_range="0 2147483647"

WORKDIR /app
COPY --from=build /app/network-monitor ./monitor

CMD ["/app/monitor"]

