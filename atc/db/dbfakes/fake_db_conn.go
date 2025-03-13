// Code generated by counterfeiter. DO NOT EDIT.
package dbfakes

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"sync"

	"github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/encryption"
)

type FakeDbConn struct {
	BeginStub        func() (db.Tx, error)
	beginMutex       sync.RWMutex
	beginArgsForCall []struct {
	}
	beginReturns struct {
		result1 db.Tx
		result2 error
	}
	beginReturnsOnCall map[int]struct {
		result1 db.Tx
		result2 error
	}
	BeginTxStub        func(context.Context, *sql.TxOptions) (db.Tx, error)
	beginTxMutex       sync.RWMutex
	beginTxArgsForCall []struct {
		arg1 context.Context
		arg2 *sql.TxOptions
	}
	beginTxReturns struct {
		result1 db.Tx
		result2 error
	}
	beginTxReturnsOnCall map[int]struct {
		result1 db.Tx
		result2 error
	}
	BusStub        func() db.NotificationsBus
	busMutex       sync.RWMutex
	busArgsForCall []struct {
	}
	busReturns struct {
		result1 db.NotificationsBus
	}
	busReturnsOnCall map[int]struct {
		result1 db.NotificationsBus
	}
	CloseStub        func() error
	closeMutex       sync.RWMutex
	closeArgsForCall []struct {
	}
	closeReturns struct {
		result1 error
	}
	closeReturnsOnCall map[int]struct {
		result1 error
	}
	ConnStub        func(context.Context) (*sql.Conn, error)
	connMutex       sync.RWMutex
	connArgsForCall []struct {
		arg1 context.Context
	}
	connReturns struct {
		result1 *sql.Conn
		result2 error
	}
	connReturnsOnCall map[int]struct {
		result1 *sql.Conn
		result2 error
	}
	DriverStub        func() driver.Driver
	driverMutex       sync.RWMutex
	driverArgsForCall []struct {
	}
	driverReturns struct {
		result1 driver.Driver
	}
	driverReturnsOnCall map[int]struct {
		result1 driver.Driver
	}
	EncryptionStrategyStub        func() encryption.Strategy
	encryptionStrategyMutex       sync.RWMutex
	encryptionStrategyArgsForCall []struct {
	}
	encryptionStrategyReturns struct {
		result1 encryption.Strategy
	}
	encryptionStrategyReturnsOnCall map[int]struct {
		result1 encryption.Strategy
	}
	ExecStub        func(string, ...any) (sql.Result, error)
	execMutex       sync.RWMutex
	execArgsForCall []struct {
		arg1 string
		arg2 []any
	}
	execReturns struct {
		result1 sql.Result
		result2 error
	}
	execReturnsOnCall map[int]struct {
		result1 sql.Result
		result2 error
	}
	ExecContextStub        func(context.Context, string, ...any) (sql.Result, error)
	execContextMutex       sync.RWMutex
	execContextArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 []any
	}
	execContextReturns struct {
		result1 sql.Result
		result2 error
	}
	execContextReturnsOnCall map[int]struct {
		result1 sql.Result
		result2 error
	}
	NameStub        func() string
	nameMutex       sync.RWMutex
	nameArgsForCall []struct {
	}
	nameReturns struct {
		result1 string
	}
	nameReturnsOnCall map[int]struct {
		result1 string
	}
	PingStub        func() error
	pingMutex       sync.RWMutex
	pingArgsForCall []struct {
	}
	pingReturns struct {
		result1 error
	}
	pingReturnsOnCall map[int]struct {
		result1 error
	}
	PrepareStub        func(string) (*sql.Stmt, error)
	prepareMutex       sync.RWMutex
	prepareArgsForCall []struct {
		arg1 string
	}
	prepareReturns struct {
		result1 *sql.Stmt
		result2 error
	}
	prepareReturnsOnCall map[int]struct {
		result1 *sql.Stmt
		result2 error
	}
	PrepareContextStub        func(context.Context, string) (*sql.Stmt, error)
	prepareContextMutex       sync.RWMutex
	prepareContextArgsForCall []struct {
		arg1 context.Context
		arg2 string
	}
	prepareContextReturns struct {
		result1 *sql.Stmt
		result2 error
	}
	prepareContextReturnsOnCall map[int]struct {
		result1 *sql.Stmt
		result2 error
	}
	QueryStub        func(string, ...any) (*sql.Rows, error)
	queryMutex       sync.RWMutex
	queryArgsForCall []struct {
		arg1 string
		arg2 []any
	}
	queryReturns struct {
		result1 *sql.Rows
		result2 error
	}
	queryReturnsOnCall map[int]struct {
		result1 *sql.Rows
		result2 error
	}
	QueryContextStub        func(context.Context, string, ...any) (*sql.Rows, error)
	queryContextMutex       sync.RWMutex
	queryContextArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 []any
	}
	queryContextReturns struct {
		result1 *sql.Rows
		result2 error
	}
	queryContextReturnsOnCall map[int]struct {
		result1 *sql.Rows
		result2 error
	}
	QueryRowStub        func(string, ...any) squirrel.RowScanner
	queryRowMutex       sync.RWMutex
	queryRowArgsForCall []struct {
		arg1 string
		arg2 []any
	}
	queryRowReturns struct {
		result1 squirrel.RowScanner
	}
	queryRowReturnsOnCall map[int]struct {
		result1 squirrel.RowScanner
	}
	QueryRowContextStub        func(context.Context, string, ...any) squirrel.RowScanner
	queryRowContextMutex       sync.RWMutex
	queryRowContextArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 []any
	}
	queryRowContextReturns struct {
		result1 squirrel.RowScanner
	}
	queryRowContextReturnsOnCall map[int]struct {
		result1 squirrel.RowScanner
	}
	SetMaxIdleConnsStub        func(int)
	setMaxIdleConnsMutex       sync.RWMutex
	setMaxIdleConnsArgsForCall []struct {
		arg1 int
	}
	SetMaxOpenConnsStub        func(int)
	setMaxOpenConnsMutex       sync.RWMutex
	setMaxOpenConnsArgsForCall []struct {
		arg1 int
	}
	StatsStub        func() sql.DBStats
	statsMutex       sync.RWMutex
	statsArgsForCall []struct {
	}
	statsReturns struct {
		result1 sql.DBStats
	}
	statsReturnsOnCall map[int]struct {
		result1 sql.DBStats
	}
	invocations      map[string][][]any
	invocationsMutex sync.RWMutex
}

func (fake *FakeDbConn) Begin() (db.Tx, error) {
	fake.beginMutex.Lock()
	ret, specificReturn := fake.beginReturnsOnCall[len(fake.beginArgsForCall)]
	fake.beginArgsForCall = append(fake.beginArgsForCall, struct {
	}{})
	stub := fake.BeginStub
	fakeReturns := fake.beginReturns
	fake.recordInvocation("Begin", []any{})
	fake.beginMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeDbConn) BeginCallCount() int {
	fake.beginMutex.RLock()
	defer fake.beginMutex.RUnlock()
	return len(fake.beginArgsForCall)
}

func (fake *FakeDbConn) BeginCalls(stub func() (db.Tx, error)) {
	fake.beginMutex.Lock()
	defer fake.beginMutex.Unlock()
	fake.BeginStub = stub
}

func (fake *FakeDbConn) BeginReturns(result1 db.Tx, result2 error) {
	fake.beginMutex.Lock()
	defer fake.beginMutex.Unlock()
	fake.BeginStub = nil
	fake.beginReturns = struct {
		result1 db.Tx
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) BeginReturnsOnCall(i int, result1 db.Tx, result2 error) {
	fake.beginMutex.Lock()
	defer fake.beginMutex.Unlock()
	fake.BeginStub = nil
	if fake.beginReturnsOnCall == nil {
		fake.beginReturnsOnCall = make(map[int]struct {
			result1 db.Tx
			result2 error
		})
	}
	fake.beginReturnsOnCall[i] = struct {
		result1 db.Tx
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) BeginTx(arg1 context.Context, arg2 *sql.TxOptions) (db.Tx, error) {
	fake.beginTxMutex.Lock()
	ret, specificReturn := fake.beginTxReturnsOnCall[len(fake.beginTxArgsForCall)]
	fake.beginTxArgsForCall = append(fake.beginTxArgsForCall, struct {
		arg1 context.Context
		arg2 *sql.TxOptions
	}{arg1, arg2})
	stub := fake.BeginTxStub
	fakeReturns := fake.beginTxReturns
	fake.recordInvocation("BeginTx", []any{arg1, arg2})
	fake.beginTxMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeDbConn) BeginTxCallCount() int {
	fake.beginTxMutex.RLock()
	defer fake.beginTxMutex.RUnlock()
	return len(fake.beginTxArgsForCall)
}

func (fake *FakeDbConn) BeginTxCalls(stub func(context.Context, *sql.TxOptions) (db.Tx, error)) {
	fake.beginTxMutex.Lock()
	defer fake.beginTxMutex.Unlock()
	fake.BeginTxStub = stub
}

func (fake *FakeDbConn) BeginTxArgsForCall(i int) (context.Context, *sql.TxOptions) {
	fake.beginTxMutex.RLock()
	defer fake.beginTxMutex.RUnlock()
	argsForCall := fake.beginTxArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeDbConn) BeginTxReturns(result1 db.Tx, result2 error) {
	fake.beginTxMutex.Lock()
	defer fake.beginTxMutex.Unlock()
	fake.BeginTxStub = nil
	fake.beginTxReturns = struct {
		result1 db.Tx
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) BeginTxReturnsOnCall(i int, result1 db.Tx, result2 error) {
	fake.beginTxMutex.Lock()
	defer fake.beginTxMutex.Unlock()
	fake.BeginTxStub = nil
	if fake.beginTxReturnsOnCall == nil {
		fake.beginTxReturnsOnCall = make(map[int]struct {
			result1 db.Tx
			result2 error
		})
	}
	fake.beginTxReturnsOnCall[i] = struct {
		result1 db.Tx
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) Bus() db.NotificationsBus {
	fake.busMutex.Lock()
	ret, specificReturn := fake.busReturnsOnCall[len(fake.busArgsForCall)]
	fake.busArgsForCall = append(fake.busArgsForCall, struct {
	}{})
	stub := fake.BusStub
	fakeReturns := fake.busReturns
	fake.recordInvocation("Bus", []any{})
	fake.busMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeDbConn) BusCallCount() int {
	fake.busMutex.RLock()
	defer fake.busMutex.RUnlock()
	return len(fake.busArgsForCall)
}

func (fake *FakeDbConn) BusCalls(stub func() db.NotificationsBus) {
	fake.busMutex.Lock()
	defer fake.busMutex.Unlock()
	fake.BusStub = stub
}

func (fake *FakeDbConn) BusReturns(result1 db.NotificationsBus) {
	fake.busMutex.Lock()
	defer fake.busMutex.Unlock()
	fake.BusStub = nil
	fake.busReturns = struct {
		result1 db.NotificationsBus
	}{result1}
}

func (fake *FakeDbConn) BusReturnsOnCall(i int, result1 db.NotificationsBus) {
	fake.busMutex.Lock()
	defer fake.busMutex.Unlock()
	fake.BusStub = nil
	if fake.busReturnsOnCall == nil {
		fake.busReturnsOnCall = make(map[int]struct {
			result1 db.NotificationsBus
		})
	}
	fake.busReturnsOnCall[i] = struct {
		result1 db.NotificationsBus
	}{result1}
}

func (fake *FakeDbConn) Close() error {
	fake.closeMutex.Lock()
	ret, specificReturn := fake.closeReturnsOnCall[len(fake.closeArgsForCall)]
	fake.closeArgsForCall = append(fake.closeArgsForCall, struct {
	}{})
	stub := fake.CloseStub
	fakeReturns := fake.closeReturns
	fake.recordInvocation("Close", []any{})
	fake.closeMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeDbConn) CloseCallCount() int {
	fake.closeMutex.RLock()
	defer fake.closeMutex.RUnlock()
	return len(fake.closeArgsForCall)
}

func (fake *FakeDbConn) CloseCalls(stub func() error) {
	fake.closeMutex.Lock()
	defer fake.closeMutex.Unlock()
	fake.CloseStub = stub
}

func (fake *FakeDbConn) CloseReturns(result1 error) {
	fake.closeMutex.Lock()
	defer fake.closeMutex.Unlock()
	fake.CloseStub = nil
	fake.closeReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeDbConn) CloseReturnsOnCall(i int, result1 error) {
	fake.closeMutex.Lock()
	defer fake.closeMutex.Unlock()
	fake.CloseStub = nil
	if fake.closeReturnsOnCall == nil {
		fake.closeReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.closeReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeDbConn) Conn(arg1 context.Context) (*sql.Conn, error) {
	fake.connMutex.Lock()
	ret, specificReturn := fake.connReturnsOnCall[len(fake.connArgsForCall)]
	fake.connArgsForCall = append(fake.connArgsForCall, struct {
		arg1 context.Context
	}{arg1})
	stub := fake.ConnStub
	fakeReturns := fake.connReturns
	fake.recordInvocation("Conn", []any{arg1})
	fake.connMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeDbConn) ConnCallCount() int {
	fake.connMutex.RLock()
	defer fake.connMutex.RUnlock()
	return len(fake.connArgsForCall)
}

func (fake *FakeDbConn) ConnCalls(stub func(context.Context) (*sql.Conn, error)) {
	fake.connMutex.Lock()
	defer fake.connMutex.Unlock()
	fake.ConnStub = stub
}

func (fake *FakeDbConn) ConnArgsForCall(i int) context.Context {
	fake.connMutex.RLock()
	defer fake.connMutex.RUnlock()
	argsForCall := fake.connArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeDbConn) ConnReturns(result1 *sql.Conn, result2 error) {
	fake.connMutex.Lock()
	defer fake.connMutex.Unlock()
	fake.ConnStub = nil
	fake.connReturns = struct {
		result1 *sql.Conn
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) ConnReturnsOnCall(i int, result1 *sql.Conn, result2 error) {
	fake.connMutex.Lock()
	defer fake.connMutex.Unlock()
	fake.ConnStub = nil
	if fake.connReturnsOnCall == nil {
		fake.connReturnsOnCall = make(map[int]struct {
			result1 *sql.Conn
			result2 error
		})
	}
	fake.connReturnsOnCall[i] = struct {
		result1 *sql.Conn
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) Driver() driver.Driver {
	fake.driverMutex.Lock()
	ret, specificReturn := fake.driverReturnsOnCall[len(fake.driverArgsForCall)]
	fake.driverArgsForCall = append(fake.driverArgsForCall, struct {
	}{})
	stub := fake.DriverStub
	fakeReturns := fake.driverReturns
	fake.recordInvocation("Driver", []any{})
	fake.driverMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeDbConn) DriverCallCount() int {
	fake.driverMutex.RLock()
	defer fake.driverMutex.RUnlock()
	return len(fake.driverArgsForCall)
}

func (fake *FakeDbConn) DriverCalls(stub func() driver.Driver) {
	fake.driverMutex.Lock()
	defer fake.driverMutex.Unlock()
	fake.DriverStub = stub
}

func (fake *FakeDbConn) DriverReturns(result1 driver.Driver) {
	fake.driverMutex.Lock()
	defer fake.driverMutex.Unlock()
	fake.DriverStub = nil
	fake.driverReturns = struct {
		result1 driver.Driver
	}{result1}
}

func (fake *FakeDbConn) DriverReturnsOnCall(i int, result1 driver.Driver) {
	fake.driverMutex.Lock()
	defer fake.driverMutex.Unlock()
	fake.DriverStub = nil
	if fake.driverReturnsOnCall == nil {
		fake.driverReturnsOnCall = make(map[int]struct {
			result1 driver.Driver
		})
	}
	fake.driverReturnsOnCall[i] = struct {
		result1 driver.Driver
	}{result1}
}

func (fake *FakeDbConn) EncryptionStrategy() encryption.Strategy {
	fake.encryptionStrategyMutex.Lock()
	ret, specificReturn := fake.encryptionStrategyReturnsOnCall[len(fake.encryptionStrategyArgsForCall)]
	fake.encryptionStrategyArgsForCall = append(fake.encryptionStrategyArgsForCall, struct {
	}{})
	stub := fake.EncryptionStrategyStub
	fakeReturns := fake.encryptionStrategyReturns
	fake.recordInvocation("EncryptionStrategy", []any{})
	fake.encryptionStrategyMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeDbConn) EncryptionStrategyCallCount() int {
	fake.encryptionStrategyMutex.RLock()
	defer fake.encryptionStrategyMutex.RUnlock()
	return len(fake.encryptionStrategyArgsForCall)
}

func (fake *FakeDbConn) EncryptionStrategyCalls(stub func() encryption.Strategy) {
	fake.encryptionStrategyMutex.Lock()
	defer fake.encryptionStrategyMutex.Unlock()
	fake.EncryptionStrategyStub = stub
}

func (fake *FakeDbConn) EncryptionStrategyReturns(result1 encryption.Strategy) {
	fake.encryptionStrategyMutex.Lock()
	defer fake.encryptionStrategyMutex.Unlock()
	fake.EncryptionStrategyStub = nil
	fake.encryptionStrategyReturns = struct {
		result1 encryption.Strategy
	}{result1}
}

func (fake *FakeDbConn) EncryptionStrategyReturnsOnCall(i int, result1 encryption.Strategy) {
	fake.encryptionStrategyMutex.Lock()
	defer fake.encryptionStrategyMutex.Unlock()
	fake.EncryptionStrategyStub = nil
	if fake.encryptionStrategyReturnsOnCall == nil {
		fake.encryptionStrategyReturnsOnCall = make(map[int]struct {
			result1 encryption.Strategy
		})
	}
	fake.encryptionStrategyReturnsOnCall[i] = struct {
		result1 encryption.Strategy
	}{result1}
}

func (fake *FakeDbConn) Exec(arg1 string, arg2 ...any) (sql.Result, error) {
	fake.execMutex.Lock()
	ret, specificReturn := fake.execReturnsOnCall[len(fake.execArgsForCall)]
	fake.execArgsForCall = append(fake.execArgsForCall, struct {
		arg1 string
		arg2 []any
	}{arg1, arg2})
	stub := fake.ExecStub
	fakeReturns := fake.execReturns
	fake.recordInvocation("Exec", []any{arg1, arg2})
	fake.execMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2...)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeDbConn) ExecCallCount() int {
	fake.execMutex.RLock()
	defer fake.execMutex.RUnlock()
	return len(fake.execArgsForCall)
}

func (fake *FakeDbConn) ExecCalls(stub func(string, ...any) (sql.Result, error)) {
	fake.execMutex.Lock()
	defer fake.execMutex.Unlock()
	fake.ExecStub = stub
}

func (fake *FakeDbConn) ExecArgsForCall(i int) (string, []any) {
	fake.execMutex.RLock()
	defer fake.execMutex.RUnlock()
	argsForCall := fake.execArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeDbConn) ExecReturns(result1 sql.Result, result2 error) {
	fake.execMutex.Lock()
	defer fake.execMutex.Unlock()
	fake.ExecStub = nil
	fake.execReturns = struct {
		result1 sql.Result
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) ExecReturnsOnCall(i int, result1 sql.Result, result2 error) {
	fake.execMutex.Lock()
	defer fake.execMutex.Unlock()
	fake.ExecStub = nil
	if fake.execReturnsOnCall == nil {
		fake.execReturnsOnCall = make(map[int]struct {
			result1 sql.Result
			result2 error
		})
	}
	fake.execReturnsOnCall[i] = struct {
		result1 sql.Result
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) ExecContext(arg1 context.Context, arg2 string, arg3 ...any) (sql.Result, error) {
	fake.execContextMutex.Lock()
	ret, specificReturn := fake.execContextReturnsOnCall[len(fake.execContextArgsForCall)]
	fake.execContextArgsForCall = append(fake.execContextArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 []any
	}{arg1, arg2, arg3})
	stub := fake.ExecContextStub
	fakeReturns := fake.execContextReturns
	fake.recordInvocation("ExecContext", []any{arg1, arg2, arg3})
	fake.execContextMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3...)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeDbConn) ExecContextCallCount() int {
	fake.execContextMutex.RLock()
	defer fake.execContextMutex.RUnlock()
	return len(fake.execContextArgsForCall)
}

func (fake *FakeDbConn) ExecContextCalls(stub func(context.Context, string, ...any) (sql.Result, error)) {
	fake.execContextMutex.Lock()
	defer fake.execContextMutex.Unlock()
	fake.ExecContextStub = stub
}

func (fake *FakeDbConn) ExecContextArgsForCall(i int) (context.Context, string, []any) {
	fake.execContextMutex.RLock()
	defer fake.execContextMutex.RUnlock()
	argsForCall := fake.execContextArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeDbConn) ExecContextReturns(result1 sql.Result, result2 error) {
	fake.execContextMutex.Lock()
	defer fake.execContextMutex.Unlock()
	fake.ExecContextStub = nil
	fake.execContextReturns = struct {
		result1 sql.Result
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) ExecContextReturnsOnCall(i int, result1 sql.Result, result2 error) {
	fake.execContextMutex.Lock()
	defer fake.execContextMutex.Unlock()
	fake.ExecContextStub = nil
	if fake.execContextReturnsOnCall == nil {
		fake.execContextReturnsOnCall = make(map[int]struct {
			result1 sql.Result
			result2 error
		})
	}
	fake.execContextReturnsOnCall[i] = struct {
		result1 sql.Result
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) Name() string {
	fake.nameMutex.Lock()
	ret, specificReturn := fake.nameReturnsOnCall[len(fake.nameArgsForCall)]
	fake.nameArgsForCall = append(fake.nameArgsForCall, struct {
	}{})
	stub := fake.NameStub
	fakeReturns := fake.nameReturns
	fake.recordInvocation("Name", []any{})
	fake.nameMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeDbConn) NameCallCount() int {
	fake.nameMutex.RLock()
	defer fake.nameMutex.RUnlock()
	return len(fake.nameArgsForCall)
}

func (fake *FakeDbConn) NameCalls(stub func() string) {
	fake.nameMutex.Lock()
	defer fake.nameMutex.Unlock()
	fake.NameStub = stub
}

func (fake *FakeDbConn) NameReturns(result1 string) {
	fake.nameMutex.Lock()
	defer fake.nameMutex.Unlock()
	fake.NameStub = nil
	fake.nameReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeDbConn) NameReturnsOnCall(i int, result1 string) {
	fake.nameMutex.Lock()
	defer fake.nameMutex.Unlock()
	fake.NameStub = nil
	if fake.nameReturnsOnCall == nil {
		fake.nameReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.nameReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeDbConn) Ping() error {
	fake.pingMutex.Lock()
	ret, specificReturn := fake.pingReturnsOnCall[len(fake.pingArgsForCall)]
	fake.pingArgsForCall = append(fake.pingArgsForCall, struct {
	}{})
	stub := fake.PingStub
	fakeReturns := fake.pingReturns
	fake.recordInvocation("Ping", []any{})
	fake.pingMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeDbConn) PingCallCount() int {
	fake.pingMutex.RLock()
	defer fake.pingMutex.RUnlock()
	return len(fake.pingArgsForCall)
}

func (fake *FakeDbConn) PingCalls(stub func() error) {
	fake.pingMutex.Lock()
	defer fake.pingMutex.Unlock()
	fake.PingStub = stub
}

func (fake *FakeDbConn) PingReturns(result1 error) {
	fake.pingMutex.Lock()
	defer fake.pingMutex.Unlock()
	fake.PingStub = nil
	fake.pingReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeDbConn) PingReturnsOnCall(i int, result1 error) {
	fake.pingMutex.Lock()
	defer fake.pingMutex.Unlock()
	fake.PingStub = nil
	if fake.pingReturnsOnCall == nil {
		fake.pingReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.pingReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeDbConn) Prepare(arg1 string) (*sql.Stmt, error) {
	fake.prepareMutex.Lock()
	ret, specificReturn := fake.prepareReturnsOnCall[len(fake.prepareArgsForCall)]
	fake.prepareArgsForCall = append(fake.prepareArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.PrepareStub
	fakeReturns := fake.prepareReturns
	fake.recordInvocation("Prepare", []any{arg1})
	fake.prepareMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeDbConn) PrepareCallCount() int {
	fake.prepareMutex.RLock()
	defer fake.prepareMutex.RUnlock()
	return len(fake.prepareArgsForCall)
}

func (fake *FakeDbConn) PrepareCalls(stub func(string) (*sql.Stmt, error)) {
	fake.prepareMutex.Lock()
	defer fake.prepareMutex.Unlock()
	fake.PrepareStub = stub
}

func (fake *FakeDbConn) PrepareArgsForCall(i int) string {
	fake.prepareMutex.RLock()
	defer fake.prepareMutex.RUnlock()
	argsForCall := fake.prepareArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeDbConn) PrepareReturns(result1 *sql.Stmt, result2 error) {
	fake.prepareMutex.Lock()
	defer fake.prepareMutex.Unlock()
	fake.PrepareStub = nil
	fake.prepareReturns = struct {
		result1 *sql.Stmt
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) PrepareReturnsOnCall(i int, result1 *sql.Stmt, result2 error) {
	fake.prepareMutex.Lock()
	defer fake.prepareMutex.Unlock()
	fake.PrepareStub = nil
	if fake.prepareReturnsOnCall == nil {
		fake.prepareReturnsOnCall = make(map[int]struct {
			result1 *sql.Stmt
			result2 error
		})
	}
	fake.prepareReturnsOnCall[i] = struct {
		result1 *sql.Stmt
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) PrepareContext(arg1 context.Context, arg2 string) (*sql.Stmt, error) {
	fake.prepareContextMutex.Lock()
	ret, specificReturn := fake.prepareContextReturnsOnCall[len(fake.prepareContextArgsForCall)]
	fake.prepareContextArgsForCall = append(fake.prepareContextArgsForCall, struct {
		arg1 context.Context
		arg2 string
	}{arg1, arg2})
	stub := fake.PrepareContextStub
	fakeReturns := fake.prepareContextReturns
	fake.recordInvocation("PrepareContext", []any{arg1, arg2})
	fake.prepareContextMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeDbConn) PrepareContextCallCount() int {
	fake.prepareContextMutex.RLock()
	defer fake.prepareContextMutex.RUnlock()
	return len(fake.prepareContextArgsForCall)
}

func (fake *FakeDbConn) PrepareContextCalls(stub func(context.Context, string) (*sql.Stmt, error)) {
	fake.prepareContextMutex.Lock()
	defer fake.prepareContextMutex.Unlock()
	fake.PrepareContextStub = stub
}

func (fake *FakeDbConn) PrepareContextArgsForCall(i int) (context.Context, string) {
	fake.prepareContextMutex.RLock()
	defer fake.prepareContextMutex.RUnlock()
	argsForCall := fake.prepareContextArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeDbConn) PrepareContextReturns(result1 *sql.Stmt, result2 error) {
	fake.prepareContextMutex.Lock()
	defer fake.prepareContextMutex.Unlock()
	fake.PrepareContextStub = nil
	fake.prepareContextReturns = struct {
		result1 *sql.Stmt
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) PrepareContextReturnsOnCall(i int, result1 *sql.Stmt, result2 error) {
	fake.prepareContextMutex.Lock()
	defer fake.prepareContextMutex.Unlock()
	fake.PrepareContextStub = nil
	if fake.prepareContextReturnsOnCall == nil {
		fake.prepareContextReturnsOnCall = make(map[int]struct {
			result1 *sql.Stmt
			result2 error
		})
	}
	fake.prepareContextReturnsOnCall[i] = struct {
		result1 *sql.Stmt
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) Query(arg1 string, arg2 ...any) (*sql.Rows, error) {
	fake.queryMutex.Lock()
	ret, specificReturn := fake.queryReturnsOnCall[len(fake.queryArgsForCall)]
	fake.queryArgsForCall = append(fake.queryArgsForCall, struct {
		arg1 string
		arg2 []any
	}{arg1, arg2})
	stub := fake.QueryStub
	fakeReturns := fake.queryReturns
	fake.recordInvocation("Query", []any{arg1, arg2})
	fake.queryMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2...)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeDbConn) QueryCallCount() int {
	fake.queryMutex.RLock()
	defer fake.queryMutex.RUnlock()
	return len(fake.queryArgsForCall)
}

func (fake *FakeDbConn) QueryCalls(stub func(string, ...any) (*sql.Rows, error)) {
	fake.queryMutex.Lock()
	defer fake.queryMutex.Unlock()
	fake.QueryStub = stub
}

func (fake *FakeDbConn) QueryArgsForCall(i int) (string, []any) {
	fake.queryMutex.RLock()
	defer fake.queryMutex.RUnlock()
	argsForCall := fake.queryArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeDbConn) QueryReturns(result1 *sql.Rows, result2 error) {
	fake.queryMutex.Lock()
	defer fake.queryMutex.Unlock()
	fake.QueryStub = nil
	fake.queryReturns = struct {
		result1 *sql.Rows
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) QueryReturnsOnCall(i int, result1 *sql.Rows, result2 error) {
	fake.queryMutex.Lock()
	defer fake.queryMutex.Unlock()
	fake.QueryStub = nil
	if fake.queryReturnsOnCall == nil {
		fake.queryReturnsOnCall = make(map[int]struct {
			result1 *sql.Rows
			result2 error
		})
	}
	fake.queryReturnsOnCall[i] = struct {
		result1 *sql.Rows
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) QueryContext(arg1 context.Context, arg2 string, arg3 ...any) (*sql.Rows, error) {
	fake.queryContextMutex.Lock()
	ret, specificReturn := fake.queryContextReturnsOnCall[len(fake.queryContextArgsForCall)]
	fake.queryContextArgsForCall = append(fake.queryContextArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 []any
	}{arg1, arg2, arg3})
	stub := fake.QueryContextStub
	fakeReturns := fake.queryContextReturns
	fake.recordInvocation("QueryContext", []any{arg1, arg2, arg3})
	fake.queryContextMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3...)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeDbConn) QueryContextCallCount() int {
	fake.queryContextMutex.RLock()
	defer fake.queryContextMutex.RUnlock()
	return len(fake.queryContextArgsForCall)
}

func (fake *FakeDbConn) QueryContextCalls(stub func(context.Context, string, ...any) (*sql.Rows, error)) {
	fake.queryContextMutex.Lock()
	defer fake.queryContextMutex.Unlock()
	fake.QueryContextStub = stub
}

func (fake *FakeDbConn) QueryContextArgsForCall(i int) (context.Context, string, []any) {
	fake.queryContextMutex.RLock()
	defer fake.queryContextMutex.RUnlock()
	argsForCall := fake.queryContextArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeDbConn) QueryContextReturns(result1 *sql.Rows, result2 error) {
	fake.queryContextMutex.Lock()
	defer fake.queryContextMutex.Unlock()
	fake.QueryContextStub = nil
	fake.queryContextReturns = struct {
		result1 *sql.Rows
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) QueryContextReturnsOnCall(i int, result1 *sql.Rows, result2 error) {
	fake.queryContextMutex.Lock()
	defer fake.queryContextMutex.Unlock()
	fake.QueryContextStub = nil
	if fake.queryContextReturnsOnCall == nil {
		fake.queryContextReturnsOnCall = make(map[int]struct {
			result1 *sql.Rows
			result2 error
		})
	}
	fake.queryContextReturnsOnCall[i] = struct {
		result1 *sql.Rows
		result2 error
	}{result1, result2}
}

func (fake *FakeDbConn) QueryRow(arg1 string, arg2 ...any) squirrel.RowScanner {
	fake.queryRowMutex.Lock()
	ret, specificReturn := fake.queryRowReturnsOnCall[len(fake.queryRowArgsForCall)]
	fake.queryRowArgsForCall = append(fake.queryRowArgsForCall, struct {
		arg1 string
		arg2 []any
	}{arg1, arg2})
	stub := fake.QueryRowStub
	fakeReturns := fake.queryRowReturns
	fake.recordInvocation("QueryRow", []any{arg1, arg2})
	fake.queryRowMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2...)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeDbConn) QueryRowCallCount() int {
	fake.queryRowMutex.RLock()
	defer fake.queryRowMutex.RUnlock()
	return len(fake.queryRowArgsForCall)
}

func (fake *FakeDbConn) QueryRowCalls(stub func(string, ...any) squirrel.RowScanner) {
	fake.queryRowMutex.Lock()
	defer fake.queryRowMutex.Unlock()
	fake.QueryRowStub = stub
}

func (fake *FakeDbConn) QueryRowArgsForCall(i int) (string, []any) {
	fake.queryRowMutex.RLock()
	defer fake.queryRowMutex.RUnlock()
	argsForCall := fake.queryRowArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeDbConn) QueryRowReturns(result1 squirrel.RowScanner) {
	fake.queryRowMutex.Lock()
	defer fake.queryRowMutex.Unlock()
	fake.QueryRowStub = nil
	fake.queryRowReturns = struct {
		result1 squirrel.RowScanner
	}{result1}
}

func (fake *FakeDbConn) QueryRowReturnsOnCall(i int, result1 squirrel.RowScanner) {
	fake.queryRowMutex.Lock()
	defer fake.queryRowMutex.Unlock()
	fake.QueryRowStub = nil
	if fake.queryRowReturnsOnCall == nil {
		fake.queryRowReturnsOnCall = make(map[int]struct {
			result1 squirrel.RowScanner
		})
	}
	fake.queryRowReturnsOnCall[i] = struct {
		result1 squirrel.RowScanner
	}{result1}
}

func (fake *FakeDbConn) QueryRowContext(arg1 context.Context, arg2 string, arg3 ...any) squirrel.RowScanner {
	fake.queryRowContextMutex.Lock()
	ret, specificReturn := fake.queryRowContextReturnsOnCall[len(fake.queryRowContextArgsForCall)]
	fake.queryRowContextArgsForCall = append(fake.queryRowContextArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 []any
	}{arg1, arg2, arg3})
	stub := fake.QueryRowContextStub
	fakeReturns := fake.queryRowContextReturns
	fake.recordInvocation("QueryRowContext", []any{arg1, arg2, arg3})
	fake.queryRowContextMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3...)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeDbConn) QueryRowContextCallCount() int {
	fake.queryRowContextMutex.RLock()
	defer fake.queryRowContextMutex.RUnlock()
	return len(fake.queryRowContextArgsForCall)
}

func (fake *FakeDbConn) QueryRowContextCalls(stub func(context.Context, string, ...any) squirrel.RowScanner) {
	fake.queryRowContextMutex.Lock()
	defer fake.queryRowContextMutex.Unlock()
	fake.QueryRowContextStub = stub
}

func (fake *FakeDbConn) QueryRowContextArgsForCall(i int) (context.Context, string, []any) {
	fake.queryRowContextMutex.RLock()
	defer fake.queryRowContextMutex.RUnlock()
	argsForCall := fake.queryRowContextArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeDbConn) QueryRowContextReturns(result1 squirrel.RowScanner) {
	fake.queryRowContextMutex.Lock()
	defer fake.queryRowContextMutex.Unlock()
	fake.QueryRowContextStub = nil
	fake.queryRowContextReturns = struct {
		result1 squirrel.RowScanner
	}{result1}
}

func (fake *FakeDbConn) QueryRowContextReturnsOnCall(i int, result1 squirrel.RowScanner) {
	fake.queryRowContextMutex.Lock()
	defer fake.queryRowContextMutex.Unlock()
	fake.QueryRowContextStub = nil
	if fake.queryRowContextReturnsOnCall == nil {
		fake.queryRowContextReturnsOnCall = make(map[int]struct {
			result1 squirrel.RowScanner
		})
	}
	fake.queryRowContextReturnsOnCall[i] = struct {
		result1 squirrel.RowScanner
	}{result1}
}

func (fake *FakeDbConn) SetMaxIdleConns(arg1 int) {
	fake.setMaxIdleConnsMutex.Lock()
	fake.setMaxIdleConnsArgsForCall = append(fake.setMaxIdleConnsArgsForCall, struct {
		arg1 int
	}{arg1})
	stub := fake.SetMaxIdleConnsStub
	fake.recordInvocation("SetMaxIdleConns", []any{arg1})
	fake.setMaxIdleConnsMutex.Unlock()
	if stub != nil {
		fake.SetMaxIdleConnsStub(arg1)
	}
}

func (fake *FakeDbConn) SetMaxIdleConnsCallCount() int {
	fake.setMaxIdleConnsMutex.RLock()
	defer fake.setMaxIdleConnsMutex.RUnlock()
	return len(fake.setMaxIdleConnsArgsForCall)
}

func (fake *FakeDbConn) SetMaxIdleConnsCalls(stub func(int)) {
	fake.setMaxIdleConnsMutex.Lock()
	defer fake.setMaxIdleConnsMutex.Unlock()
	fake.SetMaxIdleConnsStub = stub
}

func (fake *FakeDbConn) SetMaxIdleConnsArgsForCall(i int) int {
	fake.setMaxIdleConnsMutex.RLock()
	defer fake.setMaxIdleConnsMutex.RUnlock()
	argsForCall := fake.setMaxIdleConnsArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeDbConn) SetMaxOpenConns(arg1 int) {
	fake.setMaxOpenConnsMutex.Lock()
	fake.setMaxOpenConnsArgsForCall = append(fake.setMaxOpenConnsArgsForCall, struct {
		arg1 int
	}{arg1})
	stub := fake.SetMaxOpenConnsStub
	fake.recordInvocation("SetMaxOpenConns", []any{arg1})
	fake.setMaxOpenConnsMutex.Unlock()
	if stub != nil {
		fake.SetMaxOpenConnsStub(arg1)
	}
}

func (fake *FakeDbConn) SetMaxOpenConnsCallCount() int {
	fake.setMaxOpenConnsMutex.RLock()
	defer fake.setMaxOpenConnsMutex.RUnlock()
	return len(fake.setMaxOpenConnsArgsForCall)
}

func (fake *FakeDbConn) SetMaxOpenConnsCalls(stub func(int)) {
	fake.setMaxOpenConnsMutex.Lock()
	defer fake.setMaxOpenConnsMutex.Unlock()
	fake.SetMaxOpenConnsStub = stub
}

func (fake *FakeDbConn) SetMaxOpenConnsArgsForCall(i int) int {
	fake.setMaxOpenConnsMutex.RLock()
	defer fake.setMaxOpenConnsMutex.RUnlock()
	argsForCall := fake.setMaxOpenConnsArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeDbConn) Stats() sql.DBStats {
	fake.statsMutex.Lock()
	ret, specificReturn := fake.statsReturnsOnCall[len(fake.statsArgsForCall)]
	fake.statsArgsForCall = append(fake.statsArgsForCall, struct {
	}{})
	stub := fake.StatsStub
	fakeReturns := fake.statsReturns
	fake.recordInvocation("Stats", []any{})
	fake.statsMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeDbConn) StatsCallCount() int {
	fake.statsMutex.RLock()
	defer fake.statsMutex.RUnlock()
	return len(fake.statsArgsForCall)
}

func (fake *FakeDbConn) StatsCalls(stub func() sql.DBStats) {
	fake.statsMutex.Lock()
	defer fake.statsMutex.Unlock()
	fake.StatsStub = stub
}

func (fake *FakeDbConn) StatsReturns(result1 sql.DBStats) {
	fake.statsMutex.Lock()
	defer fake.statsMutex.Unlock()
	fake.StatsStub = nil
	fake.statsReturns = struct {
		result1 sql.DBStats
	}{result1}
}

func (fake *FakeDbConn) StatsReturnsOnCall(i int, result1 sql.DBStats) {
	fake.statsMutex.Lock()
	defer fake.statsMutex.Unlock()
	fake.StatsStub = nil
	if fake.statsReturnsOnCall == nil {
		fake.statsReturnsOnCall = make(map[int]struct {
			result1 sql.DBStats
		})
	}
	fake.statsReturnsOnCall[i] = struct {
		result1 sql.DBStats
	}{result1}
}

func (fake *FakeDbConn) Invocations() map[string][][]any {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.beginMutex.RLock()
	defer fake.beginMutex.RUnlock()
	fake.beginTxMutex.RLock()
	defer fake.beginTxMutex.RUnlock()
	fake.busMutex.RLock()
	defer fake.busMutex.RUnlock()
	fake.closeMutex.RLock()
	defer fake.closeMutex.RUnlock()
	fake.connMutex.RLock()
	defer fake.connMutex.RUnlock()
	fake.driverMutex.RLock()
	defer fake.driverMutex.RUnlock()
	fake.encryptionStrategyMutex.RLock()
	defer fake.encryptionStrategyMutex.RUnlock()
	fake.execMutex.RLock()
	defer fake.execMutex.RUnlock()
	fake.execContextMutex.RLock()
	defer fake.execContextMutex.RUnlock()
	fake.nameMutex.RLock()
	defer fake.nameMutex.RUnlock()
	fake.pingMutex.RLock()
	defer fake.pingMutex.RUnlock()
	fake.prepareMutex.RLock()
	defer fake.prepareMutex.RUnlock()
	fake.prepareContextMutex.RLock()
	defer fake.prepareContextMutex.RUnlock()
	fake.queryMutex.RLock()
	defer fake.queryMutex.RUnlock()
	fake.queryContextMutex.RLock()
	defer fake.queryContextMutex.RUnlock()
	fake.queryRowMutex.RLock()
	defer fake.queryRowMutex.RUnlock()
	fake.queryRowContextMutex.RLock()
	defer fake.queryRowContextMutex.RUnlock()
	fake.setMaxIdleConnsMutex.RLock()
	defer fake.setMaxIdleConnsMutex.RUnlock()
	fake.setMaxOpenConnsMutex.RLock()
	defer fake.setMaxOpenConnsMutex.RUnlock()
	fake.statsMutex.RLock()
	defer fake.statsMutex.RUnlock()
	copiedInvocations := map[string][][]any{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeDbConn) recordInvocation(key string, args []any) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]any{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]any{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ db.DbConn = new(FakeDbConn)
