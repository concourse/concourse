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
    var containers = new tree.OrderedTree();

    var steps = this.state.steps;
    var autoscroll = this.state.autoscroll;

    tree.walk(steps, function(step) {
      var stepLogs = step.logs();
      var logLines = stepLogs.lines;

      var loc = step.origin().location;

      containers.add(loc, <Step key={loc.toString()} depth={loc.length - 1} model={step} logs={logLines} autoscroll={autoscroll} />);
    });

    function recurseList(list, key) {
      return list.filterNot(function(e) {
        return e === undefined;
      }).map(function(e, i) {
        return recurse(e, key.concat([i]));
      }).toArray();
    }

    function recurse(ele, key) {
      if (Immutable.List.isList(ele)) {
        var childEles = recurseList(ele, key);

        var classes = ["nest"];


        var hasHooks = false;
        for(var i = 0; i <= ele.size - 1; i++){
          if(ele.get(i) !== undefined && ele.get(i).props !== undefined) {
            if(ele.get(i).props.model.isHook()){
              var hookClassName = "has-" + ele.get(i).props.model.hookClassName();

              if(classes.indexOf(hookClassName) == -1){
                classes.push(hookClassName);
              }
              hasHooks = true;
            }
          }
        }

        if (hasHooks){
          classes.push("hooks");
        };

        if (key.length % 2 === 0) {
          classes.push("even");
        } else {
          classes.push("odd");
        }

        return <div className={classes.join(" ")} key={key.toString()}>{childEles}</div>;
      } else {
        return <div className="seq" key={key.toString()}>{ele}</div>;
      }
    }

    return (<div className="steps">{ recurseList(containers.tree, []) }</div>);
  },
});

module.exports = Build;
