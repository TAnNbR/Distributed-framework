package trading

import (
	"fmt"
	"sync"

	"github.com/TAnNbR/Distributed-framework/actor"
)

// TickerActor 行情数据分发 Actor
type TickerActor struct {
	symbol      string
	subscribers sync.Map // map[string]*actor.PID
	lastTick    *TickerUpdate
}

// NewTickerActor 创建行情 Actor
func NewTickerActor(symbol string) actor.Producer {
	return func() actor.Receiver {
		return &TickerActor{
			symbol: symbol,
		}
	}
}

func (t *TickerActor) Receive(ctx *actor.Context) {
	switch msg := ctx.Message().(type) {
	case actor.Started:
		fmt.Printf("[Ticker-%s] 启动\n", t.symbol)

	case actor.Stopped:
		fmt.Printf("[Ticker-%s] 停止\n", t.symbol)

	case RegisterStrategy:
		// 策略订阅行情
		pid := msg.StrategyPID.(*actor.PID)
		t.subscribers.Store(pid.String(), pid)
		fmt.Printf("[Ticker-%s] 策略 %s 已订阅\n", t.symbol, pid.String())

	case TickerUpdate:
		t.lastTick = &msg
		// 广播给所有订阅者
		t.subscribers.Range(func(key, value interface{}) bool {
			pid := value.(*actor.PID)
			ctx.Send(pid, msg)
			return true
		})

	case KlineUpdate:
		// 广播K线数据
		t.subscribers.Range(func(key, value interface{}) bool {
			pid := value.(*actor.PID)
			ctx.Send(pid, msg)
			return true
		})
	}
}
