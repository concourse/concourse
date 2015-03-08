var React = require('react/addons');
var Immutable = require("immutable");
var ImmutableRenderMixin = require('react-immutable-render-mixin');

var Step = require('./step.jsx');

var Fluxxor = require('fluxxor');
var FluxMixin = Fluxxor.FluxMixin(React),
    StoreWatchMixin = Fluxxor.StoreWatchMixin;

var flux = require('./flux');

var tree = require("./tree");

var Build = React.createClass({
  mixins: [
    ImmutableRenderMixin,
    FluxMixin, StoreWatchMixin("LogsStore", "StepStore"),
  ],

  getStateFromFlux: function() {
    var flux = this.getFlux();
    return {
      logs: flux.store("LogsStore").getState(),
      steps: flux.store("StepStore").getState(),
    };
  },

  getInitialState: function() {
    return {
      autoscroll: false,
    };
  },

  render: function() {
    var containers = {};

    var logs = this.state.logs;
    var autoscroll = this.state.autoscroll;

    tree.walk(this.state.steps, function(step) {
      var loc = step.origin().location;

      var parentKey = loc.slice();
      var stepID = parentKey.pop();

      var steps = containers[parentKey.toString()];
      if (steps === undefined) {
        steps = [];
        containers[parentKey.toString()] = steps;
      }

      var stepLogs = logs.getIn(loc);

      var logLines;
      if (stepLogs !== undefined) {
        logLines = stepLogs.lines;
      } else {
        logLines = Immutable.List()
      }

      steps[stepID-1] = <Step key={loc.toString()} model={step} logs={logLines} autoscroll={autoscroll} />;
    });

    var stepEles = [];
    for (var containerLoc in containers) {
      var ele = containers[containerLoc];

      if (containerLoc.length > 0) {
        var locChain = containerLoc.split(",");
        for (var i in locChain) {
          ele = <div className="nest">{ele}</div>;
        }
      }

      stepEles.push(<div className="seq" key={containerLoc}>{ele}</div>);
    }

    return (<div className="steps">{stepEles}</div>);
  },
});

module.exports = Build;
