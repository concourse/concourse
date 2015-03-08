var Fluxxor = require('fluxxor');
var Immutable = require('immutable');
var Cursor = require('immutable/contrib/cursor');

var AnsiParser = require('node-ansiparser');

var BATCH_SIZE = 300;
var EMIT_INTERVAL = 300;

var constants = {
  ADD_LOG: 'ADD_LOG',
  ADD_ERROR: 'ADD_ERROR',

  SET_STEP_RUNNING: 'SET_STEP_RUNNING',
  SET_STEP_ERRORED: 'SET_STEP_ERRORED',
  SET_STEP_VERSION_INFO: 'SET_STEP_VERSION_INFO',
  TOGGLE_STEP_LOGS: 'TOGGLE_STEP_LOGS',
}

function LogsModel() {
  this.lines = Immutable.fromJS([[[]]]);
  this.batchCursor = Cursor.from(this.lines, function(newLines) {
    this.lines = newLines;
  }.bind(this));
  this.linesCursor = this.batchCursor.last();
  this.seqsSinceCR = 0;

  this.state = {}

  this.inst_p = function(s) {
    var textLen = s.length;
    var seqsSinceCR = this.seqsSinceCR;

    var cursor = this.cursor.update(function(line) {
      return line.slice(seqsSinceCR);
    });

    this.cursor = this.cursor.update(function(line) {
      return line.slice(0, seqsSinceCR);
    });

    this.pushSequence({
      text: s,
      foreground: this.state.foreground,
      background: this.state.background,
      bold: this.state.bold,
      italic: this.state.italic,
      underline: this.state.underline
    });

    this.seqsSinceCR++;

    cursor.forEach(function(e, i) {
      if(e.text.length >= textLen) {
        e.text = e.text.substr(textLen);
        textLen -= e.text.length;
        this.pushSequence(e);
        return false;
      }
    }, this);

    this.refreshLineCursor();
    this.changed = true;
  }

  this.inst_x = function(flag) {
    switch(flag.charCodeAt(0)) {
    case 10: // LN
      this.pushLine(Immutable.List.of({text: "\n", linebreak: true}))
      this.refreshCursor();
      this.changed = true;
      this.seqsSinceCR = 0;
      break;
    case 13: // CR
      this.seqsSinceCR = 0;
      break;
    default:
      this.inst_p(flag);
      break;
    }
  }

  this.inst_c = function(collected, params, flag) {
    for (var p in params) {
      var ansiCode = params[p];

      if (foregroundColors[ansiCode]) {
        this.state.foreground = foregroundColors[ansiCode];
      } else if (brightForegroundColors[ansiCode]) {
        this.state.foreground = brightForegroundColors[ansiCode];
      } else if (backgroundColors[ansiCode]) {
        this.state.background = backgroundColors[ansiCode];
      } else if (ansiCode == 39) {
        delete this.state.foreground;
      } else if (ansiCode == 49) {
        delete this.state.background;
      } else if (styles[ansiCode]) {
        this.state[styles[ansiCode]] = true;
      } else if (ansiCode == 22) {
        this.state.bold = false;
      } else if (ansiCode == 23) {
        this.state.italic = false;
      } else if (ansiCode == 24) {
        this.state.underline = false;
      } else if (ansiCode == 0) {
        this.state = {};
      }
    }
  }

  var ansiParser = new AnsiParser(this);

  this.addLog = function(line) {
    ansiParser.parse(line);
  };

  this.addError = function(message) {
    this.pushLine(Immutable.List())
    this.refreshCursor();
    this.pushSequence({
      text: message,
      error: true,
    });
    this.refreshLineCursor();
    this.changed = true;
  };

  this.refreshLineCursor = function() {
    this.linesCursor = this.linesCursor.set(this.linesCursor.count() - 1, this.cursor);
  };

  this.refreshCursor = function() {
    this.cursor = this.linesCursor.last();
  };

  this.pushLine = function(line) {
    if (this.linesCursor.count() >= BATCH_SIZE) {
      this.batchCursor = this.batchCursor.set(this.batchCursor.count() - 1, this.linesCursor);
      this.batchCursor = this.batchCursor.update(function(batches) {
        return batches.push(Immutable.List.of(line));
      });
      this.linesCursor = this.batchCursor.last();
    } else {
      this.linesCursor = this.linesCursor.update(function(lines) {
        return lines.push(line);
      });
    }
  };

  this.pushSequence = function(sequence) {
    this.cursor = this.cursor.update(function(line) {
      return line.push(sequence);
    });
  };

  this.refreshCursor();
  this.changed = false;
}

var LogStore = Fluxxor.createStore({
  initialize: function() {
    this.logs = Immutable.Map();

    this.bindActions(
      constants.ADD_LOG, this.onAddLog,
      constants.ADD_ERROR, this.onAddError
    );
    setInterval(this.emitEvents.bind(this), EMIT_INTERVAL);
  },

  getLogs: function(origin) {
    var logsModel = this.logs.getIn(origin.location);

    if (logsModel === undefined) {
      logsModel = new LogsModel();
      this.logs = this.logs.setIn(origin.location, logsModel);
    }

    return logsModel;
  },

  onAddLog: function(data) {
    this.getLogs(data.origin).addLog(data.line);
  },

  onAddError: function(data) {
    this.getLogs(data.origin).addError(data.line);
  },

  emitEvents: function() {
    var shouldEmit = false;
    walkTree(this.logs, function(logsModel) {
      if (logsModel.changed) {
        shouldEmit = true;
        return false;
      }
    });

    if (shouldEmit) {
      this.emit("change");

      walkTree(this.logs, function(logsModel) {
        logsModel.changed = false;
      });
    }
  },

  getState: function() {
    return this.logs;
  },
});

function walkTree(iterable, cb) {
  iterable.forEach(function(x) {
    if (Immutable.Iterable.isIterable(x)) {
      walkTree(x, cb)
    } else {
      return cb(x)
    }
  })
}

function StepModel(origin, properties) {
  this.origin = origin;

  this.showLogs = origin.type == "execute";

  this.running = false;
  this.errored = false;

  this.version = undefined;
  this.metadata = undefined;
}

var StepStore = Fluxxor.createStore({
  initialize: function() {
    this.steps = Immutable.Map();

    this.bindActions(
      constants.SET_STEP_RUNNING, this.onSetStepRunning,
      constants.SET_STEP_ERRORED, this.onSetStepErrored,
      constants.SET_STEP_VERSION_INFO, this.onSetStepVersionInfo,
      constants.TOGGLE_STEP_LOGS, this.onToggleStepLogs
    );
  },

  setStep: function(origin, changes) {
    this.steps = this.steps.updateIn(origin.location, function(stepModel) {
      if (stepModel === undefined) {
        return new StepModel(origin);
      } else {
        // *must* return new object, otherwise immutable tree remains
        // same and nothing updates

        var newStep = new StepModel(origin);

        newStep.showLogs = stepModel.showLogs;
        newStep.running = stepModel.running;
        newStep.errored = stepModel.errored;
        newStep.version = stepModel.version;
        newStep.metadata = stepModel.metadata;

        for (var k in changes) {
          if (changes.hasOwnProperty(k)) {
            newStep[k] = changes[k]
          }
        }

        return newStep;
      }
    });

    this.emit("change");
  },

  onSetStepVersionInfo: function(data) {
    this.setStep(data.origin, { version: data.version, metadata: data.metadata });
  },

  onSetStepRunning: function(data) {
    this.setStep(data.origin, { running: data.running });
  },

  onSetStepErrored: function(data) {
    this.setStep(data.origin, { errored: data.errored });
  },

  onToggleStepLogs: function(data) {
    var step = this.steps.getIn(data.origin.location);
    this.setStep(data.origin, { showLogs: !step.showLogs });
  },

  getState: function() {
    return this.steps;
  },
});

var actions = {
  addLog: function(origin, line) {
    this.dispatch(constants.ADD_LOG, { origin: origin, line: line });
  },

  addError: function(origin, line) {
    this.dispatch(constants.ADD_ERROR, { origin: origin, line: line });
  },

  setStepVersionInfo: function(origin, version, metadata) {
    this.dispatch(constants.SET_STEP_VERSION_INFO, { origin: origin, version: version, metadata: metadata});
  },

  setStepRunning: function(origin, running) {
    this.dispatch(constants.SET_STEP_RUNNING, { origin: origin, running: running });
  },

  setStepErrored: function(origin, erored) {
    this.dispatch(constants.SET_STEP_ERRORED, { origin: origin, errored: errored });
  },

  toggleStepLogs: function(origin) {
    this.dispatch(constants.TOGGLE_STEP_LOGS, { origin: origin });
  },
}

var stores = {
  "LogStore": new LogStore(),
  "StepStore": new StepStore(),
};

var foregroundColors = {
  30: 'black',
  31: 'red',
  32: 'green',
  33: 'yellow',
  34: 'blue',
  35: 'magenta',
  36: 'cyan',
  37: 'white',
};

var brightForegroundColors = {
  90: 'bright-black',
  91: 'bright-red',
  92: 'bright-green',
  93: 'bright-yellow',
  94: 'bright-blue',
  95: 'bright-magenta',
  96: 'bright-cyan',
  97: 'bright-white',
};

var backgroundColors = {
  40: 'black',
  41: 'red',
  42: 'green',
  43: 'yellow',
  44: 'blue',
  45: 'magenta',
  46: 'cyan',
  47: 'white'
};

var styles = {
  1: 'bold',
  3: 'italic',
  4: 'underline'
};

module.exports = new Fluxxor.Flux(stores, actions);
