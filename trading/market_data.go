package trading

import (
	"fmt"
	"sync"
	"time"

	"github.com/TAnNbR/Distributed-framework/actor"
)

// MarketDataActor 行情数据源 Actor
type MarketDataActor struct {
	tickers   sync.Map // symbol -> *actor.PID
	engine    *actor.Engine
	wsConn    interface{} // WebSocket 连接
	testMode  bool
	stopCh    chan struct{}
}

// MarketDataConfig 行情配置
type MarketDataConfig struct {
	Symbols  []string
	TestMode bool
}

// NewMarketDataActor 创建行情数据源 Actor
func NewMarketDataActor(engine *actor.Engine, config MarketDataConfig) actor.Producer {
	return func() actor.Receiver {
		return &MarketDataActor{
			engine:   engine,
			testMode: config.TestMode,
			stopCh:   make(chan struct{}),
		}
	}
}

func (m *MarketDataActor) Receive(ctx *actor.Context) {
	switch msg := ctx.Message().(type) {
	case actor.Started:
		fmt.Println("[MarketData] 启动")

	case actor.Stopped:
		fmt.Println("[MarketData] 停止")
		close(m.stopCh)

	case SubscribeTicker:
		m.subscribeTicker(ctx, msg.Symbol)

	case SubscribeWithStrategy:
		// 订阅行情并注册策略
		m.subscribeTicker(ctx, msg.Symbol)
		// 注册策略到 Ticker
		if tickerPID, ok := m.tickers.Load(msg.Symbol); ok {
			ctx.Send(tickerPID.(*actor.PID), RegisterStrategy{
				StrategyPID: msg.StrategyPID,
			})
		}

	case UnsubscribeTicker:
		m.unsubscribeTicker(msg.Symbol)

	case TickerUpdate:
		// 转发给对应的 Ticker Actor
		if tickerPID, ok := m.tickers.Load(msg.Symbol); ok {
			ctx.Send(tickerPID.(*actor.PID), msg)
		}
	}
}

func (m *MarketDataActor) subscribeTicker(ctx *actor.Context, symbol string) {
	// 检查是否已存在
	if _, ok := m.tickers.Load(symbol); ok {
		return
	}

	// 创建 Ticker Actor
	tickerPID := m.engine.Spawn(
		NewTickerActor(symbol),
		fmt.Sprintf("ticker-%s", symbol),
	)
	m.tickers.Store(symbol, tickerPID)

	fmt.Printf("[MarketData] 订阅: %s\n", symbol)

	if m.testMode {
		// 测试模式：生成模拟数据
		go m.generateTestData(ctx, symbol)
	} else {
		// 实盘模式：连接交易所 WebSocket
		go m.connectWebSocket(symbol)
	}
}

func (m *MarketDataActor) unsubscribeTicker(symbol string) {
	if tickerPID, ok := m.tickers.Load(symbol); ok {
		m.engine.Poison(tickerPID.(*actor.PID))
		m.tickers.Delete(symbol)
		fmt.Printf("[MarketData] 取消订阅: %s\n", symbol)
	}
}

// generateTestData 生成测试数据
func (m *MarketDataActor) generateTestData(ctx *actor.Context, symbol string) {
	basePrice := 50000.0
	if symbol == "ETH/USDT" {
		basePrice = 3000.0
	} else if symbol == "SOL/USDT" {
		basePrice = 100.0
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			// 模拟价格波动
			change := (float64(i%20) - 10) / 100 * basePrice * 0.01
			price := basePrice + change

			if tickerPID, ok := m.tickers.Load(symbol); ok {
				ctx.Send(tickerPID.(*actor.PID), TickerUpdate{
					Symbol:    symbol,
					Price:     price,
					Volume:    1000 + float64(i%100)*10,
					Bid:       price - 0.5,
					Ask:       price + 0.5,
					Timestamp: time.Now(),
				})
			}
			i++
		}
	}
}

// connectWebSocket 连接交易所 WebSocket（需要实现）
func (m *MarketDataActor) connectWebSocket(symbol string) {
	// TODO: 实现具体交易所 WebSocket 连接
	// 例如 Binance:
	// wsHandler := func(event *binance.WsKlineEvent) {
	//     m.engine.Send(m.pid, TickerUpdate{
	//         Symbol: symbol,
	//         Price:  event.Kline.Close,
	//         ...
	//     })
	// }
	// binance.WsKlineServe(symbol, "1m", wsHandler, errHandler)
}
