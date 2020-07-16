package iptables

import (
	goiptables "github.com/coreos/go-iptables/iptables"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Iptables

type Iptables interface {
	CreateChainOrFlushIfExists(table string, chain string) error
	AppendRule(table string, chain string, rulespec ...string) error
}

type iptables struct {
	goipt *goiptables.IPTables
}

var _ Iptables = (*iptables)(nil)

func New() (Iptables, error) {
	g, err := goiptables.New()
	if err != nil {
		return nil, err
	}

	ipt := iptables{
		goipt: g,
	}

	return &ipt, nil
}

func (ipt *iptables) CreateChainOrFlushIfExists(table string, chain string) error {
	err := ipt.goipt.ClearChain(table, chain)
	return err
}

func (ipt *iptables) AppendRule(table string, chain string, rulespec ...string) error {
	err := ipt.goipt.Append(table, chain, rulespec...)
	return err
}