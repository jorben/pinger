package main

import (
	"fmt"
	"github.com/go-ping/ping"
	wxworkbot "github.com/vimsucks/wxwork-bot-go"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

func init() {
	log.SetOutput(os.Stdout)
}

func main() {

	const LockFile = "/tmp/network_monitor"

	// debug 模式将打印详细日志
	debugMode, _ := strconv.ParseBool(os.Getenv("DEBUG_MODE"))

	// 获取要检测的目标地址（默认为www.baidu.com）
	address := os.Getenv("ADDRESS")
	if address == "" {
		address = "www.baidu.com"
	}

	// 获取检测周期（不小于1分钟，不大于1小时）
	cycle, _ := strconv.ParseInt(os.Getenv("INTERVAL"), 10, 64)
	if cycle < 1 {
		cycle = 1
	} else if cycle > 60 {
		cycle = 60
	}

	// 获取告警阈值，通过率低于该阈值则告警
	bottomLine, _ := strconv.ParseFloat(os.Getenv("BOTTOM_LINE"), 10)
	if bottomLine > 100 {
		// 为1时要求4次ping全通过
		bottomLine = 100
	} else if bottomLine < 25 {
		// 为0.25时要求至少1次ping通过
		bottomLine = 25
	}

	alarmTitle := os.Getenv("ALARM_TITLE")
	if alarmTitle == "" {
		alarmTitle = "Network Monitor Alarm"
	}

	// 获取企业微信机器人KEY
	botKey := os.Getenv("BOT_KEY")
	// 初始化bot
	bot := wxworkbot.New(botKey)

	// 打印初始化信息
	fmt.Println("*********** Network monitor ***********")
	fmt.Printf("ADDRESS:\t%s\n", address)
	fmt.Printf("INTERVAL:\t%d min\n", cycle)
	fmt.Printf("BOTTOM_LINE:\t%.2f\n", bottomLine)
	fmt.Printf("ALARM_TITLE:\t%s\n", alarmTitle)
	fmt.Printf("BOT_KEY:\t%s\n", botKey)
	fmt.Printf("DEBUG_MODE:\t%t\n", debugMode)
	fmt.Println("***************************************")

	// 定时器
	ticker := time.NewTicker(time.Duration(cycle) * 60 * time.Second)
	defer ticker.Stop()

	// 周期执行
	for range ticker.C {
		if debugMode {
			log.Println("== New round begin ==")
		}
		// 测试连通性，获取丢包率
		stats, err := pingAddr(address)
		if err != nil {
			log.Printf("ERROR: ping %s\n", err.Error())
			continue
		}
		if debugMode {
			log.Printf("Ping %s recv %d, loss %.2f, avgRtt %d\n",
				stats.IPAddr, stats.PacketsRecv, stats.PacketLoss, stats.AvgRtt)
		}
		passRate := 100 - stats.PacketLoss

		// 检测是否满足故障发生通知条件
		alarmContent := ""
		needAlarm := false
		if passRate < bottomLine {
			alarmContent = fmt.Sprintf("[%s]\n网络故障发生，Ping %s 通过率%.2f%%，低于阈值%.2f%%",
				alarmTitle, address, passRate, bottomLine)
			needAlarm = true
			_, err := os.Stat(LockFile)
			if err == nil {
				// 通知过了，不再通知
				needAlarm = false
			}
		} else {
			// 检查是否满足故障恢复通知条件
			_, err := os.Stat(LockFile)
			if err == nil {
				// 故障首次恢复，需要通知
				alarmContent = fmt.Sprintf("[%s]\n网络故障恢复，Ping %s 通过率%.2f%%，高于阈值%.2f%%",
					alarmTitle, address, passRate, bottomLine)
				needAlarm = true
			}
		}

		if debugMode {
			log.Printf("Need alarm:%t, content: %s\n", needAlarm,
				strings.Replace(alarmContent, "\n", " ", -1))
		}

		// 发送通知
		if needAlarm && len(bot.Key) > 0 {
			if debugMode {
				log.Printf("Send alarm to bot, key:%s\n", bot.Key)
			}
			err = alarm(bot, alarmContent, LockFile)
			if err != nil {
				log.Printf("ERROR: alarm %s\n", err.Error())
			}
		}

		if debugMode {
			log.Printf("== Round end ==")
		}
	}

}

func alarm(bot *wxworkbot.WxWorkBot, content string, fileLock string) error {

	markdown := wxworkbot.Text{
		Content: content,
	}
	err := bot.Send(markdown)
	if err != nil {
		return err
	}
	// 发送成功 则处理文件锁
	_, err = os.Stat(fileLock)
	if err == nil {
		// 文件存在 则为故障恢复，清理文件锁
		err = os.Remove(fileLock)
		return err
	} else if os.IsNotExist(err) {
		// 文件不存在 则为故障发生，需写文件锁
		f, err := os.Create(fileLock)
		_ = f.Close()
		return err
	}
	return err
}

func pingAddr(addr string) (*ping.Statistics, error) {
	pinger, err := ping.NewPinger(addr)
	if err != nil {
		return nil, err
	}

	// 执行次数
	pinger.Count = 4
	// 总执行时长限制
	pinger.Timeout = 10 * time.Second
	err = pinger.Run()
	if err != nil {
		return nil, err
	}

	return pinger.Statistics(), nil
}
