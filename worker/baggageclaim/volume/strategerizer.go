package volume

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/concourse/concourse/worker/baggageclaim"
)

type Strategerizer interface {
	StrategyFor(baggageclaim.VolumeRequest) (Strategy, error)
}

var ErrNoStrategy = errors.New("no strategy given")
var ErrUnknownStrategy = errors.New("unknown strategy")

type strategerizer struct{}

func NewStrategerizer() Strategerizer {
	return &strategerizer{}
}

func (s *strategerizer) StrategyFor(request baggageclaim.VolumeRequest) (Strategy, error) {
	if request.Strategy == nil {
		return nil, ErrNoStrategy
	}

	var strategyInfo map[string]interface{}
	err := json.Unmarshal(*request.Strategy, &strategyInfo)
	if err != nil {
		return nil, fmt.Errorf("malformed strategy: %s", err)
	}

	strategyType, ok := strategyInfo["type"].(string)
	if !ok {
		return nil, ErrUnknownStrategy
	}

	var strategy Strategy
	switch strategyType {
	case baggageclaim.StrategyEmpty:
		strategy = EmptyStrategy{}
	case baggageclaim.StrategyCopyOnWrite:
		volume, _ := strategyInfo["volume"].(string)
		strategy = COWStrategy{volume}
	case baggageclaim.StrategyImport:
		path, _ := strategyInfo["path"].(string)
		followSymlinks, _ := strategyInfo["follow_symlinks"].(bool)
		strategy = ImportStrategy{
			Path:           path,
			FollowSymlinks: followSymlinks,
		}
	default:
		return nil, ErrUnknownStrategy
	}

	return strategy, nil
}
