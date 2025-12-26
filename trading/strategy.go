package trading

import (
	"fmt"
	"time"

	"github.com/TAnNbR/Distributed-framework/actor"
)

// Strategy 策略接口
type Strategy interface {
	Name() string
	OnTick(tick TickerUpdate) *Signal
	OnKline(kline KlineUpdate) *Signal
}

// StrategyActor 策略 Actor
type StrategyActor struct {
	name        string
	strategy    Strategy
	riskManager *actor.PID
	positions   map[string]*Position
}

// NewStrategyActor 创建策略 Actor
func NewStrategyActor(name string, strategy Strategy, riskManager *actor.PID) actor.Producer {
	return func() actor.Receiver {
		return &StrategyActor{
			name:        name,
			strategy:    strategy,
			riskManager: riskManager,
			positions:   make(map[string]*Position),
		}
	}
}

func (s *StrategyActor) Receive(ctx *actor.Context) {
	switch msg := ctx.Message().(type) {
	case actor.Started:
		fmt.Printf("[Strategy-%s] 启动\n", s.name)

	case actor.Stopped:
		fmt.Printf("[Strategy-%s] 停止\n", s.name)

	case TickerUpdate:
		signal := s.strategy.OnTick(msg)
		if signal != nil {
			signal.Strategy = s.name
			signal.Timestamp = time.Now()
			// 发送风控检查
			ctx.Send(s.riskManager, RiskCheck{Signal: *signal})
		}

	case KlineUpdate:
		signal := s.strategy.OnKline(msg)
		if signal != nil {
			signal.Strategy = s.name
			signal.Timestamp = time.Now()
			ctx.Send(s.riskManager, RiskCheck{Signal: *signal})
		}

	case RiskResult:
		if msg.Approved {
			fmt.Printf("[Strategy-%s] ✅ 信号通过: %s %s %.4f @ %.2f\n",
				s.name, msg.Signal.Side, msg.Signal.Symbol,
				msg.Signal.Quantity, msg.Signal.Price)
		} else {
			fmt.Printf("[Strategy-%s] ❌ 信号拒绝: %s\n", s.name, msg.Reason)
		}

	case OrderUpdate:
		fmt.Printf("[Strategy-%s] 订单更新: %s -> %s\n",
			s.name, msg.OrderID, msg.Status)

	case Position:
		s.positions[msg.Symbol] = &msg
	}
}
