package badger

import (
	"errors"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// Engine badger引擎
type Engine struct {
	db *badger.DB // badgerDB

	gcTicker     *time.Ticker       // GC定时器
	gcInterval   time.Duration      // GC间隔时间
	gcUpdateChan chan time.Duration // GC更新间隔时间信号

	done             chan struct{} // 退出信号
	doneSuccessChain chan struct{} // 退出成功信号
	err              error         // 错误
}

// New 创建一个badger引擎
func New(opt badger.Options) (*Engine, error) {
	return new(opt)
}

// Default 创建一个默认的badger引擎
func Default(addr string) (*Engine, error) {
	return new(badger.DefaultOptions(addr))
}

// new 创建一个badger引擎
func new(opt badger.Options) (*Engine, error) {
	db, err := badger.Open(opt)
	if err != nil {
		return nil, err
	}
	be := &Engine{
		db: db,

		gcInterval:   time.Minute * 5,
		gcUpdateChan: make(chan time.Duration),

		done:             make(chan struct{}),
		doneSuccessChain: make(chan struct{}),
	}
	be.listener()
	return be, nil
}

// DB 获取badger数据库
func (e *Engine) DB() *badger.DB { return e.db }

// listener 监听取消信号
func (e *Engine) listener() {
	go e.listenerClose()
	go e.listenerGC()
}

// listenerClose 监听取消信号
func (e *Engine) listenerClose() {
	select {
	case <-e.done:
		if err := e.db.Close(); err != nil {
			e.err = err
		}
		e.db = nil
		e.gcTicker.Stop()
		e.doneSuccessChain <- struct{}{}
	}
}

// listenerGC 监听GC信号
func (e *Engine) listenerGC() {
	e.gcTicker = time.NewTicker(e.gcInterval)
	defer e.gcTicker.Stop()

	for {
		select {
		case <-e.gcTicker.C:
			e.db.RunValueLogGC(0.5)
		case newGcInterval := <-e.gcUpdateChan:
			e.updateGcInterval(newGcInterval)
		}
	}
}

// Close 关闭badger引擎
func (e *Engine) Close() error {
	e.done <- struct{}{}
	select {
	case <-e.doneSuccessChain:
		return e.err
	case <-time.After(time.Second * 5):
		e.err = errors.New("badger engine close timeout")
		return e.err
	}
}

// updateGcInterval 更新GC间隔
func (e *Engine) updateGcInterval(newGcInterval time.Duration) {
	e.gcInterval = newGcInterval
	e.gcTicker.Stop()
	e.gcTicker = time.NewTicker(e.gcInterval)
}

// SetGCInterval 设置GC间隔
func (e *Engine) SetGCInterval(interval time.Duration) {
	if 0 >= interval {
		return
	}
	e.gcUpdateChan <- interval
}
