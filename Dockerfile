FROM golang:1.20-alpine as build
WORKDIR /app
COPY ["go.mod", "go.sum", "main.go", "./"]
RUN go build

FROM alpine
# This library attempts to send an "unprivileged" ping via UDP. On Linux, this must be enabled
# RUN sysctl -w net.ipv4.ping_group_range="0 2147483647"

# 探测的目标地址，域名或者IP
ENV ADDRESS "www.baidu.com"
# 探测频率，单位分钟，范围1-60
ENV INTERVAL 1
# 连接通过率阈值，低于该阈值触发告警
ENV BOTTOM_LINE 75
# 告警信息标题
ENV ALARM_TITLE "Network Monitor Alarm"
# 企业微信群机器人KEY，例如"33b78fe3-16f7-618a-9b95-a38ee312a3f0"，不设置不会触发通知
ENV BOT_KEY ""
# 调试模式 将打印更详细的日志
ENV DEBUG_MODE "false"

WORKDIR /app
COPY --from=build /app/network-monitor ./monitor

CMD ["/app/monitor"]

