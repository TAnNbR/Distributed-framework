package strategies

import (
	"fmt"

	"github.com/TAnNbR/Distributed-framework/trading"
)

// MACrossStrategy 均线交叉策略
type MACrossStrategy struct {
	shortPeriod int
	longPeriod  int
	prices      map[string][]float64 // symbol -> prices
	positions   map[string]string    // symbol -> "long"/"short"/""
	quantity    float64
}

// NewMACrossStrategy 创建均线交叉策略
func NewMACrossStrategy(shortPeriod, longPeriod int, quantity float64) *MACrossStrategy {
	return &MACrossStrategy{
		shortPeriod: shortPeriod,
		longPeriod:  longPeriod,
		prices:      make(map[string][]float64),
		positions:   make(map[string]string),
		quantity:    quantity,
	}
}

func (s *MACrossStrategy) Name() string {
	return fmt.Sprintf("MA_%d_%d", s.shortPeriod, s.longPeriod)
}

func (s *MACrossStrategy) OnTick(tick trading.TickerUpdate) *trading.Signal {
	symbol := tick.Symbol

	// 记录价格
	if s.prices[symbol] == nil {
		s.prices[symbol] = make([]float64, 0)
	}
	s.prices[symbol] = append(s.prices[symbol], tick.Price)

	// 保留最近 longPeriod * 2 个价格
	if len(s.prices[symbol]) > s.longPeriod*2 {
		s.prices[symbol] = s.prices[symbol][1:]
	}

	// 数据不足
	if len(s.prices[symbol]) < s.longPeriod {
		return nil
	}

	// 计算均线
	shortMA := s.calcMA(symbol, s.shortPeriod)
	longMA := s.calcMA(symbol, s.longPeriod)

	currentPos := s.positions[symbol]

	// 金叉：短均线上穿长均线
	if shortMA > longMA && currentPos != "long" {
		s.positions[symbol] = "long"
		return &trading.Signal{
			Symbol:   symbol,
			Side:     "buy",
			Price:    tick.Price,
			Quantity: s.quantity,
			Reason:   fmt.Sprintf("金叉: MA%d(%.2f) > MA%d(%.2f)", s.shortPeriod, shortMA, s.longPeriod, longMA),
		}
	}

	// 死叉：短均线下穿长均线
	if shortMA < longMA && currentPos == "long" {
		s.positions[symbol] = ""
		return &trading.Signal{
			Symbol:   symbol,
			Side:     "sell",
			Price:    tick.Price,
			Quantity: s.quantity,
			Reason:   fmt.Sprintf("死叉: MA%d(%.2f) < MA%d(%.2f)", s.shortPeriod, shortMA, s.longPeriod, longMA),
		}
	}

	return nil
}

func (s *MACrossStrategy) OnKline(kline trading.KlineUpdate) *trading.Signal {
	// 使用收盘价
	tick := trading.TickerUpdate{
		Symbol: kline.Symbol,
		Price:  kline.Close,
	}
	return s.OnTick(tick)
}

func (s *MACrossStrategy) calcMA(symbol string, period int) float64 {
	prices := s.prices[symbol]
	if len(prices) < period {
		return 0
	}

	sum := 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		sum += prices[i]
	}
	return sum / float64(period)
}
