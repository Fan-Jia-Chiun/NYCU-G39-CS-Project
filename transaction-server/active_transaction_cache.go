package main

import (
	"sort"
	"sync"
)

type activeTransactionCache struct {
	mu           sync.RWMutex
	transactions map[uint]TradeInfo
}

func newActiveTransactionCache() *activeTransactionCache {
	return &activeTransactionCache{
		transactions: map[uint]TradeInfo{},
	}
}

func (c *activeTransactionCache) Add(info TradeInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.transactions[info.TransactionID] = info
}

func (c *activeTransactionCache) Remove(transactionID uint) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.transactions, transactionID)
}

func (c *activeTransactionCache) Snapshot() []TradeInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	transactionIDs := make([]uint, 0, len(c.transactions))
	for transactionID := range c.transactions {
		transactionIDs = append(transactionIDs, transactionID)
	}
	sort.Slice(transactionIDs, func(i, j int) bool {
		return transactionIDs[i] < transactionIDs[j]
	})

	result := make([]TradeInfo, 0, len(transactionIDs))
	for _, transactionID := range transactionIDs {
		result = append(result, c.transactions[transactionID])
	}

	return result
}

var currentActiveTransactions = newActiveTransactionCache()
