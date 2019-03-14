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

	log.Hooks.Add(newLagerHook(logger))
	return log
}

func newLagerHook(logger lager.Logger) *lagerHook {
	return &lagerHook{logger}
}

type lagerHook struct {
	lager.Logger
}

func (lf *lagerHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (lf *lagerHook) Fire(entry *logrus.Entry) error {
	switch entry.Level {
	case logrus.DebugLevel:
		lf.Logger.Debug("event", lager.Data{"message": entry.Message, "fields": entry.Data})
	case logrus.InfoLevel:
		lf.Logger.Info("event", lager.Data{"message": entry.Message, "fields": entry.Data})
	case logrus.WarnLevel:
		lf.Logger.Info("event", lager.Data{"message": entry.Message, "fields": entry.Data})
	case logrus.ErrorLevel:
		lf.Logger.Error("event", nil, lager.Data{"message": entry.Message, "fields": entry.Data})
	case logrus.FatalLevel:
		lf.Logger.Fatal("event", nil, lager.Data{"message": entry.Message, "fields": entry.Data})
	case logrus.PanicLevel:
		lf.Logger.Fatal("event", nil, lager.Data{"message": entry.Message, "fields": entry.Data})
	}

	return nil
}
