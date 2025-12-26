# Hollywood 量化交易系统

基于 Hollywood Actor 框架构建的量化交易系统。

## 架构

```
┌─────────────────────────────────────────────────────────────┐
│                      TradingEngine                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐     ┌──────────────┐     ┌─────────────┐ │
│  │  MarketData  │────►│   Ticker     │────►│  Strategy   │ │
│  │   (行情源)    │     │  (行情分发)   │     │   (策略)    │ │
│  └──────────────┘     └──────────────┘     └──────┬──────┘ │
│                                                    │        │
│                                                    ▼        │
│  ┌──────────────┐     ┌──────────────┐     ┌─────────────┐ │
│  │   Monitor    │     │ RiskManager  │◄────│   Signal    │ │
│  │   (监控)     │     │   (风控)     │     │   (信号)    │ │
│  └──────────────┘     └──────┬───────┘     └─────────────┘ │
│                              │                              │
│                              ▼                              │
│  ┌──────────────┐     ┌──────────────┐     ┌─────────────┐ │
│  │   Executor   │◄────│ OrderManager │────►│   Order     │ │
│  │  (执行器)    │     │  (订单管理)   │     │   (订单)    │ │
│  └──────────────┘     └──────────────┘     └─────────────┘ │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## 核心组件

| 组件 | 文件 | 职责 |
|------|------|------|
| TradingEngine | `engine.go` | 交易引擎，管理所有组件 |
| MarketData | `market_data.go` | 行情数据源，连接交易所 |
| Ticker | `ticker.go` | 行情分发，广播给策略 |
| Strategy | `strategy.go` | 策略 Actor，生成交易信号 |
| RiskManager | `risk_manager.go` | 风控管理，审核信号 |
| OrderManager | `order_manager.go` | 订单管理，创建和跟踪订单 |
| Executor | `executor.go` | 交易所执行器，下单撤单 |
| Monitor | `monitor.go` | 系统监控，统计和告警 |

## 快速开始

```go
package main

import (
    "github.com/TAnNbR/Distributed-framework/trading"
    "github.com/TAnNbR/Distributed-framework/trading/strategies"
)

func main() {
    // 创建配置
    config := trading.TradingConfig{
        TestMode: true,  // 测试模式
        RiskConfig: trading.DefaultRiskConfig(),
        Symbols: []string{"BTC/USDT"},
    }

    // 创建引擎
    engine, _ := trading.NewTradingEngine(config)

    // 添加执行器
    engine.AddExecutor("binance", "api-key", "api-secret")

    // 添加策略
    maStrategy := strategies.NewMACrossStrategy(5, 20, 0.01)
    engine.AddStrategy("MA_Cross", maStrategy, []string{"BTC/USDT"})

    // 运行...
    select {}
}
```

## 内置策略

### 1. 均线交叉策略 (MA Cross)

```go
// MA5/MA20 交叉策略，每次交易 0.01 BTC
strategy := strategies.NewMACrossStrategy(5, 20, 0.01)
```

- 金叉（短均线上穿长均线）→ 买入
- 死叉（短均线下穿长均线）→ 卖出

### 2. RSI 策略

```go
// RSI14，超买70，超卖30，每次交易 0.1 ETH
strategy := strategies.NewRSIStrategy(14, 70, 30, 0.1)
```

- RSI < 30（超卖）→ 买入
- RSI > 70（超买）→ 卖出

## 自定义策略

实现 `Strategy` 接口：

```go
type Strategy interface {
    Name() string
    OnTick(tick TickerUpdate) *Signal
    OnKline(kline KlineUpdate) *Signal
}
```

示例：

```go
type MyStrategy struct {
    // 策略状态
}

func (s *MyStrategy) Name() string {
    return "MyStrategy"
}

func (s *MyStrategy) OnTick(tick trading.TickerUpdate) *trading.Signal {
    // 你的策略逻辑
    if shouldBuy(tick) {
        return &trading.Signal{
            Symbol:   tick.Symbol,
            Side:     "buy",
            Price:    tick.Price,
            Quantity: 0.1,
            Reason:   "买入原因",
        }
    }
    return nil
}

func (s *MyStrategy) OnKline(kline trading.KlineUpdate) *trading.Signal {
    return nil
}
```

## 风控配置

```go
config := trading.RiskConfig{
    MaxPositionPct:   0.1,    // 单仓位最大占比 10%
    MaxDailyLoss:     1000,   // 日亏损限制 $1000
    MaxOrdersPerMin:  10,     // 每分钟最多 10 单
    MaxPositionValue: 10000,  // 单仓位最大 $10000
    TotalCapital:     100000, // 总资金 $100000
}
```

## 消息流

```
行情数据 → MarketData → Ticker → Strategy
                                    ↓
                                 Signal
                                    ↓
                              RiskManager
                                    ↓
                              OrderManager
                                    ↓
                               Executor
                                    ↓
                               交易所 API
```

## 运行示例

```bash
# 完整示例
go run examples/trading/main.go

# 简单测试
go run examples/trading_simple/main.go
```

## 接入实盘

1. 设置 `TestMode: false`
2. 实现 `executor.go` 中的 API 调用
3. 实现 `market_data.go` 中的 WebSocket 连接

## 注意事项

- 测试模式下不会真实下单
- 实盘前请充分测试策略
- 建议先用小资金验证
- 注意风控参数设置
