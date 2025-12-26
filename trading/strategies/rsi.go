package strategies

import (
	"fmt"
	"math"

	"github.com/TAnNbR/Distributed-framework/trading"
)

// RSIStrategy RSI 超买超卖策略
type RSIStrategy struct {
	period       int
	overbought   float64 // 超买阈值，如 70
	oversold     float64 // 超卖阈值，如 30
	prices       map[string][]float64
	positions    map[string]string
	quantity     float64
}

// NewRSIStrategy 创建 RSI 策略
func NewRSIStrategy(period int, overbought, oversold, quantity float64) *RSIStrategy {
	return &RSIStrategy{
		period:     period,
		overbought: overbought,
		oversold:   oversold,
		prices:     make(map[string][]float64),
		positions:  make(map[string]string),
		quantity:   quantity,
	}
}

func (s *RSIStrategy) Name() string {
	return fmt.Sprintf("RSI_%d", s.period)
}

func (s *RSIStrategy) OnTick(tick trading.TickerUpdate) *trading.Signal {
	symbol := tick.Symbol

	if s.prices[symbol] == nil {
		s.prices[symbol] = make([]float64, 0)
	}
	s.prices[symbol] = append(s.prices[symbol], tick.Price)

	// 保留足够的数据
	if len(s.prices[symbol]) > s.period*3 {
		s.prices[symbol] = s.prices[symbol][1:]
	}

	// 数据不足
	if len(s.prices[symbol]) < s.period+1 {
		return nil
	}

	rsi := s.calcRSI(symbol)
	currentPos := s.positions[symbol]

	// RSI 超卖，买入信号
	if rsi < s.oversold && currentPos != "long" {
		s.positions[symbol] = "long"
		return &trading.Signal{
			Symbol:   symbol,
			Side:     "buy",
			Price:    tick.Price,
			Quantity: s.quantity,
			Reason:   fmt.Sprintf("RSI 超卖: %.2f < %.2f", rsi, s.oversold),
		}
	}

	// RSI 超买，卖出信号
	if rsi > s.overbought && currentPos == "long" {
		s.positions[symbol] = ""
		return &trading.Signal{
			Symbol:   symbol,
			Side:     "sell",
			Price:    tick.Price,
			Quantity: s.quantity,
			Reason:   fmt.Sprintf("RSI 超买: %.2f > %.2f", rsi, s.overbought),
		}
	}

	return nil
}

func (s *RSIStrategy) OnKline(kline trading.KlineUpdate) *trading.Signal {
	tick := trading.TickerUpdate{
		Symbol: kline.Symbol,
		Price:  kline.Close,
	}
	return s.OnTick(tick)
}

func (s *RSIStrategy) calcRSI(symbol string) float64 {
	prices := s.prices[symbol]
	if len(prices) < s.period+1 {
		return 50 // 默认中性
	}

	var gains, losses float64
	for i := len(prices) - s.period; i < len(prices); i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			gains += change
		} else {
			losses += math.Abs(change)
		}
	}

	if losses == 0 {
		return 100
	}

	avgGain := gains / float64(s.period)
	avgLoss := losses / float64(s.period)

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}
