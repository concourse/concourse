package worker

import "sync"

type waitGroupWithCount struct {
	sync.WaitGroup

	countMutex sync.Mutex
	count      int
}

func (bwg *waitGroupWithCount) Increment() {
	bwg.Add(1)
	bwg.countMutex.Lock()
	bwg.count++
	bwg.countMutex.Unlock()
}

func (bwg *waitGroupWithCount) Decrement() {
	bwg.Done()
	bwg.countMutex.Lock()
	bwg.count--
	bwg.countMutex.Unlock()
}

func (bwg *waitGroupWithCount) Count() int {
	bwg.countMutex.Lock()
	defer bwg.countMutex.Unlock()
	return bwg.count
}
