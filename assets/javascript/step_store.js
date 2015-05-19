var Fluxxor = require("fluxxor");
var Immutable = require("immutable");
var LogsModel = require("./logs_model");

var tree = require("./tree");

var EMIT_INTERVAL = 300;

var constants = {
  ADD_LOG: 'ADD_LOG',
  ADD_ERROR: 'ADD_ERROR',
  SET_STEP_RUNNING: 'SET_STEP_RUNNING',
  SET_STEP_ERRORED: 'SET_STEP_ERRORED',
  SET_STEP_VERSION_INFO: 'SET_STEP_VERSION_INFO',
  SET_STEP_SUCCESSFUL: 'SET_STEP_SUCCESSFUL',
  TOGGLE_STEP_LOGS: 'TOGGLE_STEP_LOGS',
  PRELOAD_INPUT: 'PRELOAD_INPUT',
};

var Store = Fluxxor.createStore({
  initialize: function() {
    this.steps = Immutable.Map();

    this.preloadedInputs = Immutable.Map();

    this.bindActions(
      constants.ADD_LOG, this.onAddLog,
      constants.ADD_ERROR, this.onAddError,
      constants.SET_STEP_RUNNING, this.onSetStepRunning,
      constants.SET_STEP_ERRORED, this.onSetStepErrored,
      constants.SET_STEP_VERSION_INFO, this.onSetStepVersionInfo,
      constants.SET_STEP_SUCCESSFUL, this.onSetStepSuccessful,
      constants.TOGGLE_STEP_LOGS, this.onToggleStepLogs,
      constants.PRELOAD_INPUT, this.onPreloadInput
    );

    setInterval(this.emitChangedLogs.bind(this), EMIT_INTERVAL);
  },

  setStep: function(origin, changes) {
    var preloadedData = {};
    if (origin.type == "get" && this.preloadedInputs.has(origin.name)) {
      preloadedData = this.preloadedInputs.get(origin.name);
    }

    this.steps = this.steps.updateIn(origin.location, function(stepModel) {
      if (stepModel === undefined) {
        return new StepModel(origin).merge(preloadedData).merge(changes);
      } else {
        return stepModel.merge(changes);
      }
    });

    this.emit("change");
  },

  setPreloadedInput: function(name, data) {
    this.preloadedInputs = this.preloadedInputs.set(name, data);
  },

  onPreloadInput: function(data) {
    this.setPreloadedInput(data.name, data);
  },

  onSetStepVersionInfo: function(data) {
    this.setStep(data.origin, { version: data.version, metadata: data.metadata });
  },

  onSetStepSuccessful: function(data) {
    this.setStep(data.origin, { successful: data.successful });
  },

  onSetStepRunning: function(data) {
    this.setStep(data.origin, { running: data.running });
  },

  onSetStepErrored: function(data) {
    this.setStep(data.origin, { errored: data.errored });
  },

  onToggleStepLogs: function(data) {
    var step = this.steps.getIn(data.origin.location);
    this.setStep(data.origin, { showLogs: !step.isShowingLogs(), userToggled: true });
  },

  onAddLog: function(data) {
    var step = this.steps.getIn(data.origin.location);
    if (step) {
      step.logs().addLog(data.line);
    }
  },

  onAddError: function(data) {
    var step = this.steps.getIn(data.origin.location);
    if (step) {
      step.logs().addError(data.line);
    }
  },

  emitChangedLogs: function() {
    var stepsToUpdate = [];
    var emitChange = false;
    tree.walk(this.steps, function(step) {
      var logs = step.logs();
      if (logs.changed) {
        stepsToUpdate.push(step);
        emitChange = true;

        // reset
        logs.changed = false;
      }
    });

    for (var i in stepsToUpdate) {
      var step = stepsToUpdate[i];
      this.steps = this.steps.setIn(step.origin().location, step.copy());
    }

    if (emitChange) {
      this.emit("change");
    }
  },

  getState: function() {
    return this.steps;
  },
});

function StepModel(origin) {
  this._map = new Immutable.Map({
    origin: origin,

    logs: new LogsModel(),
    showLogs: true,
    userToggled: false,

    running: false,
    errored: false,

    version: undefined,
    metadata: undefined,

    successful: undefined,

    firstOccurrence: false,
  });

  this.merge = function(attrs) {
    var newMap = this._map.merge(attrs);
    if (newMap == this._map) {
      return this;
    }

    var newModel = new StepModel(this.origin());
    newModel._map = newMap;
    return newModel;
  }

  this.copy = function() {
    var newModel = new StepModel(this.origin());
    newModel._map = this._map;
    return newModel;
  }

  this.origin = function() {
    return this._map.get("origin");
  }

  this.logs = function() {
    return this._map.get("logs");
  }

  this.isShowingLogs = function() {
    var showLogs = this._map.get("showLogs");
    if (this.wasToggled()) {
      return showLogs
    }

    return showLogs && (this.isRunning() || this.isErrored() || this.isSuccessful() === false);
  }

  this.isRunning = function() {
    return this._map.get("running");
  }

  this.isErrored = function() {
    return this._map.get("errored");
  }

  this.isSubStep = function() {
    return !!this.origin().substep;
  }

  this.isSuccessful = function() {
    return this._map.get("successful");
  }

  this.isFirstOccurrence = function() {
    return this._map.get("firstOccurrence");
  }

  this.wasToggled = function() {
    return this._map.get("userToggled");
  }

  this.version = function() {
    var x = this._map.get("version");
    if (!x) {
      return undefined;
    }

    return x.toJS();
  }

  this.metadata = function() {
    var meta = this._map.get("metadata");
    if (meta === undefined) {
      return undefined;
    }

    return meta.toJS();
  }
}

module.exports = {
  Store: Store
};

for (var k in constants) {
  module.exports[k] = constants[k];
}
