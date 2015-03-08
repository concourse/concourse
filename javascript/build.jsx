var React = require('react/addons');
var Immutable = require("immutable");
var ImmutableRenderMixin = require('react-immutable-render-mixin');

var Logs = require('./logs.jsx');
var Step = require('./step.jsx');

var Fluxxor = require('fluxxor');
var FluxMixin = Fluxxor.FluxMixin(React),
    StoreWatchMixin = Fluxxor.StoreWatchMixin;
var flux = require('./stores');

window.React = React;

var $ = require('jquery');

function walkTree(iterable, cb) {
  iterable.forEach(function(x) {
    if (Immutable.Iterable.isIterable(x)) {
      walkTree(x, cb)
    } else {
      return cb(x)
    }
  })
}

var Build = React.createClass({
  mixins: [
    ImmutableRenderMixin,
    FluxMixin, StoreWatchMixin("LogStore", "StepStore"),
  ],

  getStateFromFlux: function() {
    var flux = this.getFlux();
    return {
      logs: flux.store("LogStore").getState(),
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

    walkTree(this.state.steps, function(step) {
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

var buildComponent = React.render(<Build flux={flux}/>,
                                  document.getElementById('build-logs'));

function streamLog(uri) {
  var es = new EventSource(uri);

  var successfullyConnected = false;

  es.addEventListener("event", function(event) {
    var msg = JSON.parse(event.data);

    var versionSegments = msg.version.split(".");

    var majorHandler = eventHandlers[versionSegments[0]];
    if (!majorHandler) {
      console.log("unknown major version: " + msg.version);
      return;
    }

    var minorHandler = majorHandler[versionSegments[1]] || majorHandler["*"];
    if (!minorHandler) {
      console.log("unknown minor version: " + msg.version);
      return;
    }

    var handler = minorHandler[msg.event];
    if (!handler) {
      return;
    }

    handler(msg.data);
  });

  es.addEventListener("end", function(event) {
    es.close();
  });

  es.onerror = function(event) {
    if(!successfullyConnected) {
      // assume rejected because of auth
      $("#build-requires-auth").show();
    }
  };
}

var moment = require('moment');
require("moment-duration-format");

var legacyInputLocations = [];
var legacyOutputLocations = [];

function legacyInputOrigin(name) {
  var inputOrigin = legacyInputLocations.indexOf(name);
  if (inputOrigin == -1) {
    legacyInputLocations.push(name);
    inputOrigin = legacyInputLocations.length - 1;
  }

  var loc = [1, inputOrigin + 1];

  return {
    "name": name,
    "type": "get",
    "location": loc
  }
}

function legacyRunOrigin() {
  return {
    "name": "build",
    "type": "execute",
    "location": [2]
  }
}

function legacyOutputOrigin(name) {
  var outputOrigin = legacyOutputLocations.indexOf(name);
  if (outputOrigin == -1) {
    legacyOutputLocations.push(name);
    outputOrigin = legacyOutputLocations.length - 1;
  }

  var loc = [3, outputOrigin + 1];

  return {
    "name": name,
    "type": "put",
    "location": loc
  }
}

var eventHandlers = {
  "1": {
    "*": {
      "log": function(data) {
        processLogs(data);
      },

      "error": function(data) {
        processError(data);
      },

      "status": function(data) {
        processStatus(data);
      },

      "input": function(data) {
        var resource = event["input"];

        var origin = legacyInputOrigin(resource.name);

        flux.actions.setStepVersionInfo(origin, resource.version, resource.metadata);
        flux.actions.setStepRunning(origin, false);
      },

      "output": function(data) {
        var resource = event["output"];

        var origin = legacyOutputOrigin(resource.name);

        flux.actions.setStepVersionInfo(origin, resource.version, resource.metadata);
        flux.actions.setStepRunning(origin, false);
      },

      "finish": function(data) {
        var origin = legacyRunOrigin();
        flux.actions.setStepRunning(origin, false);
      }
    }
  },

  "2": {
    "*": {
      "input": function(data) {
        var origin = legacyInputOrigin(data.plan.name);

        flux.actions.setStepVersionInfo(origin, data.version, data.metadata);
        flux.actions.setStepRunning(origin, false);
      },

      "output": function(data) {
        var origin = legacyOutputOrigin(data.plan.name);

        flux.actions.setStepVersionInfo(origin, data.version, data.metadata);
        flux.actions.setStepRunning(origin, false);
      }
    }
  }
}

function processStatus(event) {
  var currentStatus = $("#page-header").attr("class");

  var buildTimes = $(".build-times");

  var status = event.status;
  var m = moment.unix(event.time);

  var time = $("<time>");
  time.text(m.fromNow());
  time.attr("datetime", m.format());
  time.attr("title", m.format("lll Z"));
  time.addClass(status);

  if(status == "started") {
    $("<dt/>").text(status).appendTo(buildTimes);
    $("<dd/>").append(time).appendTo(buildTimes);
  } else {
    $("<dt/>").text(status).appendTo(buildTimes);
    $("<dd/>").append(time).appendTo(buildTimes);

    var startTime = $(".build-times time.started").attr("datetime");

    // Some events cause the build to never start (e.g. input errors).
    var didStart = !!startTime

    if(didStart) {
      var duration = moment.duration(m.diff(moment(startTime)));

      var durationEle = $("<span>");
      durationEle.addClass("duration");
      durationEle.text(duration.format("h[h]m[m]s[s]"));

      $("<dt/>").text("duration").appendTo(buildTimes);
      $("<dd/>").append(durationEle).appendTo(buildTimes);
    }
  }

  // only transition from transient states; state may already be set
  // if the page loaded after build was done
  if(currentStatus != "pending" && currentStatus != "started") {
    return;
  }

  $("#page-header").attr("class", status);
  $("#builds .current").attr("class", status + " current");

  if(status != "started") {
    $(".abort-build").remove();
  }
}

function processLogs(event) {
  var origin;

  switch(event.origin.type) {
  case "run":
    origin = legacyRunOrigin();
    break;
  case "input":
    origin = legacyInputOrigin(event.origin.name);
    break;
  case "output":
    origin = legacyOutputOrigin(event.origin.name);
    break;
  }

  if(!origin || !event.payload) {
    return;
  }

  flux.actions.setStepRunning(origin, true);
  flux.actions.addLog(origin, event.payload);
}

function processError(event) {
  var origin;

  if(event.origin && event.origin.type) {
    switch(event.origin.type) {
    case "input":
      origin = legacyOutputOrigin(event.origin.name);
      break;
    case "output":
      origin = legacyOutputOrigin(event.origin.name);
      break;
    }
  } else {
    origin = legacyRunOrigin();
  }

  if(!origin) {
    return;
  }

  flux.actions.setStepRunning(origin, false);
  flux.actions.setStepErrored(origin, true);
  flux.actions.addError(origin, event.message);
}

function scrollToCurrentBuild() {
  var currentBuild = $("#builds .current");
  var buildWidth = currentBuild.width();
  var left = currentBuild.offset().left;

  if((left + buildWidth) > window.innerWidth) {
    $("#builds").scrollLeft(left - buildWidth);
  }
}

$(document).ready(function() {
  var title = $("#page-header");

  if (title.hasClass("pending") || title.hasClass("started")) {
    buildComponent.setState({ autoscroll: true });
  }

  $(window).scroll(function() {
    var scrollEnd = $(window).scrollTop() + $(window).height();

    if (scrollEnd >= ($(document).height() - 16)) {
      buildComponent.setState({ autoscroll: true });
    } else {
      buildComponent.setState({ autoscroll: false });
    }
  });

  $("#builds").bind('mousewheel', function(e){
    if (e.originalEvent.deltaX != 0) {
      $(this).scrollLeft($(this).scrollLeft() + e.originalEvent.deltaX);
    } else {
      $(this).scrollLeft($(this).scrollLeft() - e.originalEvent.deltaY);
    }

    return false;
  });

  scrollToCurrentBuild();
});

window.streamLog = streamLog;
