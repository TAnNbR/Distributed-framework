package trading

import (
	"fmt"
	"time"

	"github.com/TAnNbR/Distributed-framework/actor"
)

// MonitorActor 系统监控 Actor
type MonitorActor struct {
	stats *SystemStats
}

// SystemStats 系统统计
type SystemStats struct {
	StartTime      time.Time
	TotalSignals   int64
	ApprovedSignals int64
	RejectedSignals int64
	TotalOrders    int64
	FilledOrders   int64
	CanceledOrders int64
	TotalPnL       float64
}

// NewMonitorActor 创建监控 Actor
func NewMonitorActor() actor.Producer {
	return func() actor.Receiver {
		return &MonitorActor{
			stats: &SystemStats{
				StartTime: time.Now(),
			},
		}
	}
}

func (m *MonitorActor) Receive(ctx *actor.Context) {
	switch msg := ctx.Message().(type) {
	case actor.Started:
		fmt.Println("[Monitor] 启动")
		// 定时打印统计
		ctx.SendRepeat(ctx.PID(), PrintStats{}, 30*time.Second)

	case actor.Stopped:
		fmt.Println("[Monitor] 停止")
		m.printStats()

	// 监听系统事件
	case actor.ActorStartedEvent:
		fmt.Printf("[Monitor] Actor 启动: %s\n", msg.PID.String())

	case actor.ActorStoppedEvent:
		fmt.Printf("[Monitor] Actor 停止: %s\n", msg.PID.String())

	case actor.ActorRestartedEvent:
		fmt.Printf("[Monitor] ⚠️ Actor 重启: %s (原因: %v)\n", msg.PID.String(), msg.Reason)

	case actor.DeadLetterEvent:
		fmt.Printf("[Monitor] ⚠️ 死信: 目标=%s, 消息=%T\n", msg.Target.String(), msg.Message)

	// 业务事件
	case RiskCheck:
		m.stats.TotalSignals++

	case RiskResult:
		if msg.Approved {
			m.stats.ApprovedSignals++
		} else {
			m.stats.RejectedSignals++
		}

	case Order:
		m.stats.TotalOrders++

	case OrderUpdate:
		switch msg.Status {
		case "filled":
			m.stats.FilledOrders++
		case "canceled":
			m.stats.CanceledOrders++
		}

	case PrintStats:
		m.printStats()
	}
}

// PrintStats 打印统计消息
type PrintStats struct{}

func (m *MonitorActor) printStats() {
	uptime := time.Since(m.stats.StartTime)
	fmt.Println("\n========== 系统统计 ==========")
	fmt.Printf("运行时间: %s\n", uptime.Round(time.Second))
	fmt.Printf("信号总数: %d (通过: %d, 拒绝: %d)\n",
		m.stats.TotalSignals, m.stats.ApprovedSignals, m.stats.RejectedSignals)
	fmt.Printf("订单总数: %d (成交: %d, 取消: %d)\n",
		m.stats.TotalOrders, m.stats.FilledOrders, m.stats.CanceledOrders)
	fmt.Printf("总盈亏: $%.2f\n", m.stats.TotalPnL)
	fmt.Println("==============================\n")
}
