var Fluxxor = require('fluxxor');
var LogsStore = require("./logs_store");
var StepStore = require("./step_store");

var actions = {
  preloadInput: function(name, firstOccurrence, version, metadata) {
    this.dispatch(StepStore.PRELOAD_INPUT, {
      name: name,
      firstOccurrence: firstOccurrence,
      version: version,
      metadata: metadata
    });
  },

  addLog: function(origin, line) {
    this.dispatch(LogsStore.ADD_LOG, { origin: origin, line: line });
  },

  addError: function(origin, line) {
    this.dispatch(LogsStore.ADD_ERROR, { origin: origin, line: line });
  },

  setStepVersionInfo: function(origin, version, metadata) {
    this.dispatch(StepStore.SET_STEP_VERSION_INFO, { origin: origin, version: version, metadata: metadata });
  },

  setStepSuccessful: function(origin, successful) {
    this.dispatch(StepStore.SET_STEP_SUCCESSFUL, { origin: origin, successful: successful });
  },

  setStepRunning: function(origin, running) {
    this.dispatch(StepStore.SET_STEP_RUNNING, { origin: origin, running: running });
  },

  setStepErrored: function(origin, errored) {
    this.dispatch(StepStore.SET_STEP_ERRORED, { origin: origin, errored: errored });
  },

  toggleStepLogs: function(origin) {
    this.dispatch(StepStore.TOGGLE_STEP_LOGS, { origin: origin });
  },
};

var stores = {
  "LogsStore": new LogsStore.Store(),
  "StepStore": new StepStore.Store(),
};

module.exports = new Fluxxor.Flux(stores, actions);
