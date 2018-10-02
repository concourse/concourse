package beacon

import "sync"

type waitGroupWithCount struct {
	*sync.WaitGroup
	countMutex *sync.Mutex
	count int
}

func (bwg *waitGroupWithCount) Increment () {
	bwg.Add(1)
	bwg.countMutex.Lock()
	defer bwg.countMutex.Unlock()
	bwg.count++
}

func (bwg *waitGroupWithCount) Decrement () {
	bwg.Done()
	bwg.countMutex.Lock()
	defer bwg.countMutex.Unlock()
	bwg.count--
}

func (bwg *waitGroupWithCount) Count () int {
	bwg.countMutex.Lock()
	defer bwg.countMutex.Unlock()
	return bwg.count
}
