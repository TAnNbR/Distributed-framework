package trading

import (
	"fmt"
	"time"

	"github.com/TAnNbR/Distributed-framework/actor"
)

// ExecutorActor 交易所执行器 Actor
type ExecutorActor struct {
	exchange     string
	orderManager *actor.PID
	apiKey       string
	apiSecret    string
	testMode     bool // 测试模式，不真实下单
}

// ExecutorConfig 执行器配置
type ExecutorConfig struct {
	Exchange     string
	APIKey       string
	APISecret    string
	TestMode     bool
	OrderManager *actor.PID
}

// NewExecutorActor 创建执行器 Actor
func NewExecutorActor(config ExecutorConfig) actor.Producer {
	return func() actor.Receiver {
		return &ExecutorActor{
			exchange:     config.Exchange,
			orderManager: config.OrderManager,
			apiKey:       config.APIKey,
			apiSecret:    config.APISecret,
			testMode:     config.TestMode,
		}
	}
}

func (e *ExecutorActor) Receive(ctx *actor.Context) {
	switch msg := ctx.Message().(type) {
	case actor.Started:
		mode := "实盘"
		if e.testMode {
			mode = "测试"
		}
		fmt.Printf("[Executor-%s] 启动 (%s模式)\n", e.exchange, mode)

	case actor.Stopped:
		fmt.Printf("[Executor-%s] 停止\n", e.exchange)

	case Order:
		e.executeOrder(ctx, msg)

	case CancelOrder:
		e.cancelOrder(ctx, msg)
	}
}

func (e *ExecutorActor) executeOrder(ctx *actor.Context, order Order) {
	fmt.Printf("[Executor-%s] 执行订单: %s %s %s %.4f @ %.2f\n",
		e.exchange, order.ID[:8], order.Side, order.Symbol, order.Quantity, order.Price)

	if e.testMode {
		// 测试模式：模拟成交
		go func() {
			time.Sleep(100 * time.Millisecond) // 模拟延迟

			// 模拟订单确认
			ctx.Send(e.orderManager, OrderUpdate{
				OrderID:   order.ID,
				Status:    "open",
				Timestamp: time.Now(),
			})

			time.Sleep(500 * time.Millisecond) // 模拟成交延迟

			// 模拟完全成交
			ctx.Send(e.orderManager, OrderUpdate{
				OrderID:   order.ID,
				Status:    "filled",
				FilledQty: order.Quantity,
				AvgPrice:  order.Price,
				Timestamp: time.Now(),
			})
		}()
	} else {
		// 实盘模式：调用交易所 API
		go func() {
			err := e.placeOrderAPI(order)
			if err != nil {
				ctx.Send(e.orderManager, OrderUpdate{
					OrderID:   order.ID,
					Status:    "failed",
					Timestamp: time.Now(),
				})
				fmt.Printf("[Executor-%s] ❌ 下单失败: %v\n", e.exchange, err)
			}
		}()
	}
}

func (e *ExecutorActor) cancelOrder(ctx *actor.Context, cancel CancelOrder) {
	fmt.Printf("[Executor-%s] 取消订单: %s\n", e.exchange, cancel.OrderID[:8])

	if e.testMode {
		ctx.Send(e.orderManager, OrderUpdate{
			OrderID:   cancel.OrderID,
			Status:    "canceled",
			Timestamp: time.Now(),
		})
	} else {
		// 实盘：调用取消 API
		go func() {
			err := e.cancelOrderAPI(cancel.OrderID)
			if err != nil {
				fmt.Printf("[Executor-%s] ❌ 取消失败: %v\n", e.exchange, err)
			}
		}()
	}
}

// placeOrderAPI 调用交易所下单 API（需要实现）
func (e *ExecutorActor) placeOrderAPI(order Order) error {
	// TODO: 实现具体交易所 API 调用
	// 例如 Binance:
	// client := binance.NewClient(e.apiKey, e.apiSecret)
	// _, err := client.NewCreateOrderService().
	//     Symbol(order.Symbol).
	//     Side(order.Side).
	//     Type(order.Type).
	//     Quantity(order.Quantity).
	//     Price(order.Price).
	//     Do(context.Background())
	return nil
}

// cancelOrderAPI 调用交易所取消 API（需要实现）
func (e *ExecutorActor) cancelOrderAPI(orderID string) error {
	// TODO: 实现具体交易所 API 调用
	return nil
}
