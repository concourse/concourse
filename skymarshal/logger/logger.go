package logger

import (
	"io/ioutil"

	"code.cloudfoundry.org/lager"
	"github.com/sirupsen/logrus"
)

func New(logger lager.Logger) *logrus.Logger {
	var log = &logrus.Logger{
		Out:       ioutil.Discard,
		Hooks:     make(logrus.LevelHooks),
		Formatter: new(logrus.JSONFormatter),
		Level:     logrus.DebugLevel,
	}

	log.Hooks.Add(NewLagerHook(logger))
	return log
}

func NewLagerHook(logger lager.Logger) *lagerHook {
	return &lagerHook{logger}
}

type lagerHook struct {
	lager.Logger
}

func (self *lagerHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (self *lagerHook) Fire(entry *logrus.Entry) error {
	switch entry.Level {
	case logrus.DebugLevel:
		self.Logger.Debug("event", lager.Data{"message": entry.Message, "fields": entry.Data})
		break
	case logrus.InfoLevel:
		self.Logger.Info("event", lager.Data{"message": entry.Message, "fields": entry.Data})
		break
	case logrus.WarnLevel:
		self.Logger.Info("event", lager.Data{"message": entry.Message, "fields": entry.Data})
		break
	case logrus.ErrorLevel:
		self.Logger.Error("event", nil, lager.Data{"message": entry.Message, "fields": entry.Data})
		break
	case logrus.FatalLevel:
		self.Logger.Fatal("event", nil, lager.Data{"message": entry.Message, "fields": entry.Data})
		break
	case logrus.PanicLevel:
		self.Logger.Fatal("event", nil, lager.Data{"message": entry.Message, "fields": entry.Data})
		break
	}
	return nil
}
