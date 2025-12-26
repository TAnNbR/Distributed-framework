package trading

import "time"

// ==================== 行情消息 ====================

// TickerUpdate 行情更新
type TickerUpdate struct {
	Symbol    string
	Price     float64
	Volume    float64
	Bid       float64
	Ask       float64
	Timestamp time.Time
}

// KlineUpdate K线更新
type KlineUpdate struct {
	Symbol    string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	Interval  string // "1m", "5m", "1h"
	Timestamp time.Time
}

// SubscribeTicker 订阅行情
type SubscribeTicker struct {
	Symbol string
}

// UnsubscribeTicker 取消订阅
type UnsubscribeTicker struct {
	Symbol string
}

// SubscribeWithStrategy 订阅行情并注册策略
type SubscribeWithStrategy struct {
	Symbol      string
	StrategyPID interface{} // *actor.PID
}

// ==================== 交易信号 ====================

// Signal 交易信号
type Signal struct {
	ID        string
	Symbol    string
	Side      string  // "buy" / "sell"
	Price     float64
	Quantity  float64
	Strategy  string
	Reason    string
	Timestamp time.Time
}

// ==================== 订单消息 ====================

// Order 订单
type Order struct {
	ID         string
	Symbol     string
	Side       string  // "buy" / "sell"
	Type       string  // "limit" / "market"
	Price      float64
	Quantity   float64
	FilledQty  float64
	Status     string // "pending" / "open" / "filled" / "canceled"
	Exchange   string
	Strategy   string
	CreateTime time.Time
	UpdateTime time.Time
}

// OrderUpdate 订单状态更新
type OrderUpdate struct {
	OrderID   string
	Status    string
	FilledQty float64
	AvgPrice  float64
	Timestamp time.Time
}

// CancelOrder 取消订单
type CancelOrder struct {
	OrderID string
}

// ==================== 风控消息 ====================

// RiskCheck 风控检查请求
type RiskCheck struct {
	Signal Signal
}

// RiskResult 风控检查结果
type RiskResult struct {
	Approved bool
	Reason   string
	Signal   Signal
}

// ==================== 仓位消息 ====================

// Position 仓位
type Position struct {
	Symbol    string
	Side      string  // "long" / "short"
	Quantity  float64
	AvgPrice  float64
	PnL       float64
	UpdatedAt time.Time
}

// PositionQuery 查询仓位
type PositionQuery struct {
	Symbol string
}

// ==================== 注册消息 ====================

// RegisterStrategy 注册策略到行情
type RegisterStrategy struct {
	StrategyPID interface{} // *actor.PID
	Symbols     []string
}

// RegisterExecutor 注册交易所执行器
type RegisterExecutor struct {
	Exchange string
	PID      interface{} // *actor.PID
}
