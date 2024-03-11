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
			log.Printf("Ping %s, send %d, recv %d, loss %.2f%%\n",
				stats.IPAddr, stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss)
		}
		passRate := 100 - stats.PacketLoss

		// 检测是否满足故障发生通知条件
		alarmContent := ""
		needAlarm := false
		locked := isLocked(LockFile)

		if passRate < bottomLine {
			alarmContent = fmt.Sprintf("[%s]\n网络故障发生，Ping %s 通过率 %.2f%%，低于阈值 %.2f%%",
				alarmTitle, address, passRate, bottomLine)
			if !locked {
				// 无文件锁，说明首次故障，需通知
				needAlarm = true
			}
		} else {
			alarmContent = fmt.Sprintf("[%s]\n网络故障恢复，Ping %s 通过率 %.2f%%，高于阈值 %.2f%%",
				alarmTitle, address, passRate, bottomLine)
			if locked {
				// 有文件锁，说明在故障中，需要通知故障恢复
				needAlarm = true
			}
		}

		if debugMode {
			log.Printf("Need alarm: %t\n", needAlarm)
		}

		// 发送通知
		if needAlarm && len(bot.Key) > 0 {
			if debugMode {
				log.Printf("Send alarm to bot, key: %s, Msg: %s\n",
					bot.Key, strings.Replace(alarmContent, "\n", " ", -1))
			}
			err = alarm(bot, alarmContent)
			if err == nil {
				// 消息发送成功 翻转文件锁
				err = swLock(LockFile, !locked)
				if err != nil {
					log.Printf("ERROR: swLock %s\n", err.Error())
				}
			} else {
				log.Printf("ERROR: alarm %s\n", err.Error())
			}
		}

		if debugMode {
			log.Printf("== Round end ==")
		}
	}

}

// 检查文件锁是否存在
func isLocked(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		// 文件存在
		return true
	} else if !os.IsNotExist(err) {
		log.Printf("Stat Error, path: %s, err: %s\n", path, err.Error())
	}
	return false
}

// 处理文件锁
func swLock(path string, lock bool) error {
	if lock {
		f, err := os.Create(path)
		_ = f.Close()
		return err
	} else {
		err := os.Remove(path)
		return err
	}
}

// 发送企业微信机器人消息
func alarm(bot *wxworkbot.WxWorkBot, content string) error {
	markdown := wxworkbot.Text{
		Content: content,
	}
	return bot.Send(markdown)
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
