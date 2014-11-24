var Fluxxor = require('fluxxor');
var Immutable = require('immutable');
var Cursor = require('immutable/contrib/cursor');

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
  this.carriageReturned = false;
  this.lines = Immutable.fromJS([[[]]]);
  this.batchCursor = Cursor.from(this.lines, function(newLines) {
    this.lines = newLines;
  }.bind(this));
  this.linesCursor = this.batchCursor.last();
  this.carriage = 0;

  this.addLog = function(line) {
    var sequence = ansiparse(line);

    for (var i = 0; i < sequence.length; i++) {
      if (sequence[i].cr) {
        this.carriage = 0;
      } else if (sequence[i].linebreak) {
        this.pushLine(Immutable.List.of(sequence[i]))
        this.refreshCursor();
        this.carriage = 0;
        this.changed = true;
      } else if (sequence[i].text) {
        var textLen = sequence[i].text.length;
        var carriage = this.carriage;
        var cursor = this.cursor.update(function(line) {
          return line.slice(carriage);
        });
        this.cursor = this.cursor.update(function(line) {
          return line.slice(0, carriage);
        });
        this.pushSequence(sequence[i]);
        this.carriage++;

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
    };
  };

  this.addError = function(message) {
    this.pushLine(Immutable.List())
    this.refreshCursor();
    this.pushSequence({
      text: message,
      error: true,
    });
    this.refreshLineCursor();
    this.carriageReturned = false;
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

module.exports = new Fluxxor.Flux(stores, actions);
