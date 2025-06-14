package badger

import (
	"bytes"
	"encoding/gob"
	"os"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// BadgerTX 事务函数
type BadgerTX func(tx *badger.Txn) error

// TxSet 事务设置参数操作
func (e *Engine) TxSet(tx BadgerTX) error {
	return e.db.Update(tx)
}

// TxGet 事务获取参数操作
func (e *Engine) TxGet(tx BadgerTX) error {
	return e.db.View(tx)
}

// Set 设置参数
func (e *Engine) Set(key, value []byte) error {
	return e.TxSet(func(tx *badger.Txn) error {
		return tx.Set(key, value)
	})
}

// SetAny 设置任意参数
// 通过gob序列化数据后存储
func (e *Engine) SetAny(key, value any) error {
	var keyBuf bytes.Buffer
	var valBuf bytes.Buffer
	if err := gob.NewEncoder(&keyBuf).Encode(key); err != nil {
		return err
	}
	if err := gob.NewEncoder(&valBuf).Encode(value); err != nil {
		return err
	}
	return e.Set(keyBuf.Bytes(), valBuf.Bytes())
}

// Get 获取参数
func (e *Engine) Get(key []byte) ([]byte, error) {
	var value []byte
	err := e.TxGet(func(tx *badger.Txn) error {
		item, err := tx.Get(key)
		if err != nil {
			return err
		}
		value, err = item.ValueCopy(nil)
		return err
	})
	return value, err
}

// GetAny 获取任意参数
// 通过gob反序列化数据后获取结果写入valuePoint
func (e *Engine) GetAny(key any, valuePoint any) error {
	var keyBuf bytes.Buffer
	if err := gob.NewEncoder(&keyBuf).Encode(key); err != nil {
		return err
	}
	valueBytes, err := e.Get(keyBuf.Bytes())
	if err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewReader(valueBytes)).Decode(valuePoint)
}

// Del 删除参数
func (e *Engine) Del(key []byte) error {
	return e.TxSet(func(tx *badger.Txn) error {
		return tx.Delete(key)
	})
}

// DelAny 删除任意参数
// 通过gob反序列化数据后获取结果删除
func (e *Engine) DelAny(key any) error {
	var keyBuf bytes.Buffer
	if err := gob.NewEncoder(&keyBuf).Encode(key); err != nil {
		return err
	}
	return e.Del(keyBuf.Bytes())
}

// SetTTL 设置参数的过期时间
func (e *Engine) SetTTL(key, value []byte, ttl time.Duration) error {
	return e.TxSet(func(tx *badger.Txn) error {
		return tx.SetEntry(badger.NewEntry(key, value).WithTTL(ttl))
	})
}

// SetAnyTTL 设置参数的过期时间
func (e *Engine) SetAnyTTL(key, value []byte, ttl time.Duration) error {
	var keyBuf bytes.Buffer
	var valBuf bytes.Buffer
	if err := gob.NewEncoder(&keyBuf).Encode(key); err != nil {
		return err
	}
	if err := gob.NewEncoder(&valBuf).Encode(value); err != nil {
		return err
	}
	return e.SetTTL(keyBuf.Bytes(), valBuf.Bytes(), ttl)
}

// Batch 批量操作
type BadgerBatch func(*badger.WriteBatch) error

// Batch 批量操作
func (e *Engine) Batch(bb BadgerBatch) error {
	wb := e.db.NewWriteBatch()
	defer wb.Cancel()
	return bb(wb)
}

// Backup 备份数据库
func (e *Engine) Backup(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = e.db.Backup(f, 0); err != nil {
		return err
	}
	return nil
}

// GetKey 获取所有key
// @param prefix 前缀
func (e *Engine) GetKey(prefix []byte) ([][]byte, error) {
	var keys [][]byte

	err := e.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // 只获取键，不获取值

		it := txn.NewIterator(opts)
		defer it.Close()

		if prefix == nil {
			// 查询所有键
			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				key := item.KeyCopy(nil)
				keys = append(keys, key)
			}
		} else {
			// 查询指定前缀的键
			prefixBytes := prefix
			for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
				item := it.Item()
				key := item.KeyCopy(nil)
				keys = append(keys, key)
			}
		}

		return nil
	})

	return keys, err
}

// Exists 判断key是否存在
func (e *Engine) Exists(key []byte) (bool, error) {
	var exists bool
	err := e.TxGet(func(tx *badger.Txn) error {
		_, err := tx.Get(key)
		if err == nil {
			exists = true
			return nil
		}
		if err == badger.ErrKeyNotFound {
			exists = false
			return nil
		}
		return err
	})
	return exists, err
}

// LoadMessage 加载消息
type LoadMessage func(err error)

// Load 加载备份数据
// 用户需使用一个LoadMessage函数处理错误
// 该函数为异步函数, 不建议用户在主程序中调用
func (e *Engine) Load(filename string, lm LoadMessage) {
	lmErr := make(chan error)

	go func() {
		f, err := os.Open(filename)
		if err != nil {
			lmErr <- err
			return
		}
		defer f.Close()
		lmErr <- e.db.Load(f, 500)
	}()

	if lm != nil {
		go func() {
			select {
			case err := <-lmErr:
				lm(err)
			}
		}()
	}
}
