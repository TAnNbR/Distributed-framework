package trading

import (
	"fmt"
	"sync"
	"time"

	"github.com/TAnNbR/Distributed-framework/actor"
)

// RiskConfig 风控配置
type RiskConfig struct {
	MaxPositionPct   float64 // 单个仓位最大占比
	MaxDailyLoss     float64 // 日最大亏损
	MaxOrdersPerMin  int     // 每分钟最大订单数
	MaxPositionValue float64 // 单个仓位最大价值
	TotalCapital     float64 // 总资金
}

// DefaultRiskConfig 默认风控配置
func DefaultRiskConfig() RiskConfig {
	return RiskConfig{
		MaxPositionPct:   0.1,    // 10%
		MaxDailyLoss:     1000,   // $1000
		MaxOrdersPerMin:  10,     // 10 单/分钟
		MaxPositionValue: 10000,  // $10000
		TotalCapital:     100000, // $100000
	}
}

// RiskManagerActor 风控管理 Actor
type RiskManagerActor struct {
	config       RiskConfig
	orderManager *actor.PID
	dailyPnL     float64
	positions    sync.Map // symbol -> Position
	orderTimes   []time.Time
	mu           sync.Mutex
}

// NewRiskManagerActor 创建风控管理 Actor
func NewRiskManagerActor(config RiskConfig, orderManager *actor.PID) actor.Producer {
	return func() actor.Receiver {
		return &RiskManagerActor{
			config:       config,
			orderManager: orderManager,
			orderTimes:   make([]time.Time, 0),
		}
	}
}

func (r *RiskManagerActor) Receive(ctx *actor.Context) {
	switch msg := ctx.Message().(type) {
	case actor.Started:
		fmt.Println("[RiskManager] 启动")
		fmt.Printf("[RiskManager] 配置: 最大仓位 %.0f%%, 日亏损限制 $%.0f\n",
			r.config.MaxPositionPct*100, r.config.MaxDailyLoss)

	case actor.Stopped:
		fmt.Println("[RiskManager] 停止")

	case RiskCheck:
		result := r.checkRisk(msg.Signal)

		// 回复策略
		ctx.Respond(result)

		// 通过风控，发送给订单管理
		if result.Approved {
			ctx.Send(r.orderManager, msg.Signal)
		}

	case Position:
		r.positions.Store(msg.Symbol, msg)

	case OrderUpdate:
		// 更新日盈亏
		// 实际应该根据成交价格计算
	}
}

func (r *RiskManagerActor) checkRisk(signal Signal) RiskResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. 检查日亏损限制
	if r.dailyPnL <= -r.config.MaxDailyLoss {
		return RiskResult{
			Approved: false,
			Reason:   fmt.Sprintf("超过日亏损限制: %.2f", r.dailyPnL),
			Signal:   signal,
		}
	}

	// 2. 检查订单频率
	now := time.Now()
	r.cleanOldOrders(now)
	if len(r.orderTimes) >= r.config.MaxOrdersPerMin {
		return RiskResult{
			Approved: false,
			Reason:   fmt.Sprintf("订单频率过高: %d/min", len(r.orderTimes)),
			Signal:   signal,
		}
	}

	// 3. 检查仓位大小
	positionValue := signal.Price * signal.Quantity
	if positionValue > r.config.MaxPositionValue {
		return RiskResult{
			Approved: false,
			Reason:   fmt.Sprintf("仓位过大: $%.2f > $%.2f", positionValue, r.config.MaxPositionValue),
			Signal:   signal,
		}
	}

	// 4. 检查仓位占比
	positionPct := positionValue / r.config.TotalCapital
	if positionPct > r.config.MaxPositionPct {
		return RiskResult{
			Approved: false,
			Reason:   fmt.Sprintf("仓位占比过高: %.2f%% > %.2f%%", positionPct*100, r.config.MaxPositionPct*100),
			Signal:   signal,
		}
	}

	// 记录订单时间
	r.orderTimes = append(r.orderTimes, now)

	return RiskResult{
		Approved: true,
		Signal:   signal,
	}
}

func (r *RiskManagerActor) cleanOldOrders(now time.Time) {
	cutoff := now.Add(-time.Minute)
	newTimes := make([]time.Time, 0)
	for _, t := range r.orderTimes {
		if t.After(cutoff) {
			newTimes = append(newTimes, t)
		}
	}
	r.orderTimes = newTimes
}
