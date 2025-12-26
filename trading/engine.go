package trading

import (
	"fmt"

	"github.com/TAnNbR/Distributed-framework/actor"
)

// TradingEngine 量化交易引擎
type TradingEngine struct {
	engine       *actor.Engine
	marketData   *actor.PID
	orderManager *actor.PID
	riskManager  *actor.PID
	monitor      *actor.PID
	strategies   map[string]*actor.PID
	executors    map[string]*actor.PID
	config       TradingConfig
}

// TradingConfig 交易引擎配置
type TradingConfig struct {
	TestMode   bool
	RiskConfig RiskConfig
	Symbols    []string
}

// DefaultTradingConfig 默认配置
func DefaultTradingConfig() TradingConfig {
	return TradingConfig{
		TestMode:   true,
		RiskConfig: DefaultRiskConfig(),
		Symbols:    []string{"BTC/USDT"},
	}
}

// NewTradingEngine 创建交易引擎
func NewTradingEngine(config TradingConfig) (*TradingEngine, error) {
	// 创建 Actor 引擎
	engine, err := actor.NewEngine(actor.NewEngineConfig())
	if err != nil {
		return nil, fmt.Errorf("创建引擎失败: %w", err)
	}

	te := &TradingEngine{
		engine:     engine,
		strategies: make(map[string]*actor.PID),
		executors:  make(map[string]*actor.PID),
		config:     config,
	}

	// 初始化核心组件
	te.initComponents()

	return te, nil
}

func (te *TradingEngine) initComponents() {
	// 1. 创建监控
	te.monitor = te.engine.Spawn(NewMonitorActor(), "monitor")
	te.engine.Subscribe(te.monitor) // 订阅系统事件

	// 2. 创建订单管理器
	te.orderManager = te.engine.Spawn(NewOrderManagerActor(), "order-manager")

	// 3. 创建风控管理器
	te.riskManager = te.engine.Spawn(
		NewRiskManagerActor(te.config.RiskConfig, te.orderManager),
		"risk-manager",
	)

	// 4. 创建行情数据源
	te.marketData = te.engine.Spawn(
		NewMarketDataActor(te.engine, MarketDataConfig{
			Symbols:  te.config.Symbols,
			TestMode: te.config.TestMode,
		}),
		"market-data",
	)

	fmt.Println("[TradingEngine] 核心组件初始化完成")
}

// AddExecutor 添加交易所执行器
func (te *TradingEngine) AddExecutor(exchange, apiKey, apiSecret string) *actor.PID {
	executorPID := te.engine.Spawn(
		NewExecutorActor(ExecutorConfig{
			Exchange:     exchange,
			APIKey:       apiKey,
			APISecret:    apiSecret,
			TestMode:     te.config.TestMode,
			OrderManager: te.orderManager,
		}),
		fmt.Sprintf("executor-%s", exchange),
	)

	te.executors[exchange] = executorPID

	// 注册到订单管理器
	te.engine.Send(te.orderManager, RegisterExecutor{
		Exchange: exchange,
		PID:      executorPID,
	})

	return executorPID
}

// AddStrategy 添加策略
func (te *TradingEngine) AddStrategy(name string, strategy Strategy, symbols []string) *actor.PID {
	strategyPID := te.engine.Spawn(
		NewStrategyActor(name, strategy, te.riskManager),
		fmt.Sprintf("strategy-%s", name),
	)

	te.strategies[name] = strategyPID

	// 注册策略到订单管理器
	te.engine.Send(te.orderManager, RegisterStrategy{
		StrategyPID: strategyPID,
		Symbols:     symbols,
	})

	// 订阅行情并注册策略
	for _, symbol := range symbols {
		te.engine.Send(te.marketData, SubscribeWithStrategy{
			Symbol:      symbol,
			StrategyPID: strategyPID,
		})
	}

	return strategyPID
}

// SubscribeSymbol 订阅交易对
func (te *TradingEngine) SubscribeSymbol(symbol string) {
	te.engine.Send(te.marketData, SubscribeTicker{Symbol: symbol})
}

// UnsubscribeSymbol 取消订阅
func (te *TradingEngine) UnsubscribeSymbol(symbol string) {
	te.engine.Send(te.marketData, UnsubscribeTicker{Symbol: symbol})
}

// Stop 停止引擎
func (te *TradingEngine) Stop() {
	fmt.Println("[TradingEngine] 正在停止...")

	// 停止策略
	for name, pid := range te.strategies {
		te.engine.Poison(pid)
		fmt.Printf("[TradingEngine] 停止策略: %s\n", name)
	}

	// 停止执行器
	for exchange, pid := range te.executors {
		te.engine.Poison(pid)
		fmt.Printf("[TradingEngine] 停止执行器: %s\n", exchange)
	}

	// 停止核心组件
	te.engine.Poison(te.marketData)
	te.engine.Poison(te.riskManager)
	te.engine.Poison(te.orderManager)
	te.engine.Poison(te.monitor)

	fmt.Println("[TradingEngine] 已停止")
}

// Engine 获取底层 Actor 引擎
func (te *TradingEngine) Engine() *actor.Engine {
	return te.engine
}

// GetStrategy 获取策略 PID
func (te *TradingEngine) GetStrategy(name string) *actor.PID {
	return te.strategies[name]
}

// GetExecutor 获取执行器 PID
func (te *TradingEngine) GetExecutor(exchange string) *actor.PID {
	return te.executors[exchange]
}
