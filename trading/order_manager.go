package trading

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/TAnNbR/Distributed-framework/actor"
)

// OrderManagerActor 订单管理 Actor
type OrderManagerActor struct {
	executors  sync.Map          // exchange -> *actor.PID
	orders     sync.Map          // orderID -> *Order
	strategies sync.Map          // strategy -> *actor.PID
}

// NewOrderManagerActor 创建订单管理 Actor
func NewOrderManagerActor() actor.Producer {
	return func() actor.Receiver {
		return &OrderManagerActor{}
	}
}

func (o *OrderManagerActor) Receive(ctx *actor.Context) {
	switch msg := ctx.Message().(type) {
	case actor.Started:
		fmt.Println("[OrderManager] 启动")

	case actor.Stopped:
		fmt.Println("[OrderManager] 停止")

	case RegisterExecutor:
		// 注册交易所执行器
		pid := msg.PID.(*actor.PID)
		o.executors.Store(msg.Exchange, pid)
		fmt.Printf("[OrderManager] 注册执行器: %s\n", msg.Exchange)

	case RegisterStrategy:
		// 注册策略，用于回调
		pid := msg.StrategyPID.(*actor.PID)
		for _, symbol := range msg.Symbols {
			o.strategies.Store(symbol, pid)
		}

	case Signal:
		// 收到信号，创建订单
		order := o.createOrder(msg)
		o.orders.Store(order.ID, order)

		fmt.Printf("[OrderManager] 创建订单: %s %s %s %.4f @ %.2f\n",
			order.ID[:8], order.Side, order.Symbol, order.Quantity, order.Price)

		// 发送给执行器
		o.sendToExecutor(ctx, order)

	case OrderUpdate:
		// 更新订单状态
		if orderVal, ok := o.orders.Load(msg.OrderID); ok {
			order := orderVal.(*Order)
			order.Status = msg.Status
			order.FilledQty = msg.FilledQty
			order.UpdateTime = msg.Timestamp

			fmt.Printf("[OrderManager] 订单更新: %s -> %s (成交: %.4f)\n",
				msg.OrderID[:8], msg.Status, msg.FilledQty)

			// 通知策略
			if strategyPID, ok := o.strategies.Load(order.Symbol); ok {
				ctx.Send(strategyPID.(*actor.PID), msg)
			}
		}

	case CancelOrder:
		// 取消订单
		if orderVal, ok := o.orders.Load(msg.OrderID); ok {
			order := orderVal.(*Order)
			if executorPID, ok := o.executors.Load(order.Exchange); ok {
				ctx.Send(executorPID.(*actor.PID), msg)
			}
		}
	}
}

func (o *OrderManagerActor) createOrder(signal Signal) *Order {
	return &Order{
		ID:         generateOrderID(),
		Symbol:     signal.Symbol,
		Side:       signal.Side,
		Type:       "limit",
		Price:      signal.Price,
		Quantity:   signal.Quantity,
		Status:     "pending",
		Exchange:   "binance", // 默认交易所
		Strategy:   signal.Strategy,
		CreateTime: time.Now(),
	}
}

func (o *OrderManagerActor) sendToExecutor(ctx *actor.Context, order *Order) {
	if executorPID, ok := o.executors.Load(order.Exchange); ok {
		ctx.Send(executorPID.(*actor.PID), *order)
	} else {
		fmt.Printf("[OrderManager] ⚠️ 未找到执行器: %s\n", order.Exchange)
	}
}


// generateOrderID 生成订单ID
func generateOrderID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Intn(10000))
}
