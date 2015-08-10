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
    FluxMixin, StoreWatchMixin("StepStore"),
  ],

  getStateFromFlux: function() {
    var flux = this.getFlux();
    return {
      steps: flux.store("StepStore").getState(),
    };
  },

  getInitialState: function() {
    return {
      autoscroll: false,
    };
  },

  render: function() {
    var containers = this.state.steps.getRenderableData();

    var steps = this.state.steps;
    var autoscroll = this.state.autoscroll;

    var rootSteps = []

    for(var i = 0; i < containers.length; i++ ){
      currentStep = containers[i]

      if (currentStep == undefined) {
        continue;
      }

      if(currentStep.location.parent_id === 0){
        rootSteps.push(buildStep(currentStep));
      }
    }

    function buildStep(step) {
      var classes = ["seq"]
      var childSteps = []

      for(var i = 0; i <= step.children.length; i++){
        childStep = step.children[i]

        if(childStep == undefined){
          continue
        }

        childSteps.push(buildStep(childStep))
      }

      if(step.step.isHook()){
        classes.push("hook");
        classes.push(step.step.hookClassName());
      }

      if(step.step.isDependentGet()){
        classes.push("seq-dependent-get");
      }

      if(step.group){
        var groupSteps = []
        var groupClasses = []

        if (step.aggregate) {
          groupClasses.push("aggregate")
        } else {
          groupClasses.push("serial")
        }

        for(var i = 0; i <= step.groupSteps.length; i++){
          groupStep = step.groupSteps[i]

          if(groupStep == undefined){
            continue
          }

          groupSteps.push(buildStep(groupStep))
        }

        return <div className={classes.join(' ')} key={step.key}><div className={groupClasses.join(' ')}>{groupSteps}</div> <div className="children">{childSteps}</div></div>
      }

      return <div className={classes.join(' ')} key={step.key}><Step key={step.key} model={step.step} logs={step.logLines} autoscroll={autoscroll} /><div className="children">{childSteps}</div></div>
    }

    return (<div className="steps">{rootSteps}</div>);
  },
});

module.exports = Build;
