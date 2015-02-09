var Fluxxor = require('fluxxor');
var Immutable = require('immutable');
var Cursor = require('immutable/contrib/cursor');

var AnsiParser = require('node-ansiparser');

var BATCH_SIZE = 300;
var EMIT_INTERVAL = 300;

var constants = {
  ADD_LOG: 'ADD_LOG',
  ADD_RESOURCE: 'ADD_RESOURCE',
  ADD_ERROR: 'ADD_ERROR',
  SET_RESOURCE_RUNNING: 'SET_RESOURCE_RUNNING',
  SET_RESOURCE_ERRORED: 'SET_RESOURCE_ERRORED',
  TOGGLE_RESOURCE_LOGS: 'TOGGLE_RESOURCE_LOGS',
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
    this.logs = {};
    this.bindActions(
      constants.ADD_LOG, this.onAddLog,
      constants.ADD_ERROR, this.onAddError
    );
    setInterval(this.emitEvents.bind(this), EMIT_INTERVAL);
  },

  getLogs: function(type) {
    if (this.logs[type] === undefined) {
      this.logs[type] = new LogsModel();
    }
    return this.logs[type];
  },

  onAddLog: function(data) {
    this.getLogs(data.type).addLog(data.line);
  },

  onAddError: function(data) {
    this.getLogs(data.type).addError(data.line);
  },

  emitEvents: function() {
    var shouldEmit = false;

    for (var k in this.logs) {
      if (this.logs.hasOwnProperty(k)) {
        if (this.logs[k].changed) {
          shouldEmit = true;
          break;
        }
      }
    }

    if (shouldEmit) {
      this.emit("change");

      for (var k in this.logs) {
        if (this.logs.hasOwnProperty(k)) {
          this.logs[k].changed = false
        }
      }
    }
  },

  getState: function() {
    var state = {};
    for (var k in this.logs) {
      if (this.logs.hasOwnProperty(k)) {
        state[k] = this.logs[k].lines;
      }
    }
    return Immutable.fromJS(state);
  },
});

var ResourceStore = Fluxxor.createStore({
  initialize: function() {
    this.resources = {};
    this.bindActions(
      constants.ADD_RESOURCE, this.onAddResource,
      constants.SET_RESOURCE_RUNNING, this.onSetResourceRunning,
      constants.SET_RESOURCE_ERRORED, this.onSetResourceErrored,
      constants.TOGGLE_RESOURCE_LOGS, this.onToggleResourceLogs
    );
  },

  addResource: function(type, resource) {
    if (this.resources[type] === undefined) {
      this.resources[type] = Immutable.Map();
    }
    resource.kind = type;
    var newResource = Immutable.fromJS(resource);
    if (this.resources[type].has(resource.name)) {
      newResource = this.resources[type].get(resource.name).merge(newResource);
    }
    this.setResource(type, resource.name, newResource);
    this.emit("change");
    return this.resources[type].get(resource.name);
  },

  updateResource: function(type, name, attributes) {
    attributes.name = name;
    this.addResource(type, attributes);
  },

  setResource: function(type, name, resource) {
    this.resources[type] = this.resources[type].set(name, resource);
  },

  onAddResource: function(data) {
    this.addResource(data.type, data.resource);
  },

  onSetResourceRunning: function(data) {
    this.updateResource(data.type, data.name, { running: data.value });
  },

  onSetResourceErrored: function(data) {
    this.updateResource(data.type, data.name, { errored: data.value });
  },

  onToggleResourceLogs: function(data) {
    var resource = this.resources[data.type].get(data.name);
    this.updateResource(data.type, data.name, { showLogs: !resource.get('showLogs') });
  },

  getState: function() {
    return Immutable.fromJS(this.resources);
  },
});

var actions = {
  addLog: function(type, line) {
    this.dispatch(constants.ADD_LOG, { type: type, line: line });
  },

  addError: function(type, line) {
    this.dispatch(constants.ADD_ERROR, { type: type, line: line });
  },

  addResource: function(type, resource) {
    this.dispatch(constants.ADD_RESOURCE, { type: type, resource: resource });
  },

  setResourceRunning: function(type, name, value) {
    this.dispatch(constants.SET_RESOURCE_RUNNING, { type: type, name: name, value: value });
  },

  setResourceErrored: function(type, name, value) {
    this.dispatch(constants.SET_RESOURCE_ERRORED, { type: type, name: name, value: value });
  },

  toggleResourceLogs: function(type, name) {
    this.dispatch(constants.TOGGLE_RESOURCE_LOGS, { type: type, name: name });
  },
}

var stores = {
  LogStore: new LogStore(),
  ResourceStore: new ResourceStore(),
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
