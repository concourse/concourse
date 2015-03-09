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
    var containers = new tree.OrderedTree();

    var steps = this.state.steps;
    var logs = this.state.logs;
    var autoscroll = this.state.autoscroll;

    tree.walk(steps, function(step) {
      var loc = step.origin().location;

      var stepLogs = logs.getIn(loc);

      var logLines;
      if (stepLogs !== undefined) {
        logLines = stepLogs.lines;
      } else {
        logLines = Immutable.List()
      }

      containers.add(loc, <Step key={loc.toString()} depth={loc.length - 1} model={step} logs={logLines} autoscroll={autoscroll} />);
    });

    var stepEles = [];
    containers.walk(function(ele) {
      var key = ele.key;

      if (ele.props.depth > 0) {
        for (var i = 0; i < ele.props.depth; i++) {
          ele = <div className="nest">{ele}</div>;
        }
      }

      stepEles.push(<div className="seq" key={key}>{ele}</div>);
    });

    return (<div className="steps">{stepEles}</div>);
  },
});

module.exports = Build;
