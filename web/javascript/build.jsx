var React = require('react/addons');
var ImmutableRenderMixin = require('react-immutable-render-mixin');

var Logs = require('./logs.jsx');
var Resource = require('./resource.jsx');

var Fluxxor = require('fluxxor');
var FluxMixin = Fluxxor.FluxMixin(React),
    StoreWatchMixin = Fluxxor.StoreWatchMixin;
var flux = require('./stores');

window.React = React;

var $ = require('jquery');

var Build = React.createClass({
  mixins: [
    ImmutableRenderMixin, FluxMixin,
    StoreWatchMixin("LogStore", "ResourceStore"),
  ],

  getStateFromFlux: function() {
    var flux = this.getFlux();
    return {
      logs: flux.store("LogStore").getState(),
      resources: flux.store("ResourceStore").getState(),
    };
  },

  getInitialState: function() {
    return {
      autoscroll: false,
    };
  },

  render: function() {
    var t = new Date();
    var runLogs = '';
    if (this.state.logs.has('run')) {
      runLogs = <Logs autoscroll={this.state.autoscroll} batches={this.state.logs.get('run')} />;
    }
    var inputResources = [];

    var resources = this.state.resources;

    if (resources.get('input') !== undefined) {
      resources.get('input').forEach(function(input) {
        var logs;
        var key = 'input-' + input.get('name');
        if (this.state.logs.has(key)) {
          logs = this.state.logs.get(key);
        }
        inputResources.push(<Resource key={key} resource={input} logs={logs} autoscroll={this.state.autoscroll} />);
      }, this);
    }

    var outputResources = [];
    if (resources.get('output') !== undefined) {
      resources.get('output').forEach(function(output) {
        var logs;
        var key = 'output-' + output.get('name');
        if (this.state.logs.has(key)) {
          logs = this.state.logs.get(key);
        }
        outputResources.push(<Resource key={key} resource={output} logs={logs} autoscroll={this.state.autoscroll} />);
      }, this);
    }

    return (
      <div>
        <div className="build-resources build-inputs">{inputResources}</div>
        <div className="build-logs">{runLogs}</div>
        <div className="build-resources build-outputs">{outputResources}</div>
      </div>
    );
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
        renderResource(data, "input");
      },

      "output": function(data) {
        renderResource(data, "output");
      }
    }
  },

  "2": {
    "*": {
      "input": function(data) {
        flux.actions.addResource("input", {
          "name": data.plan.name,
          "version": data.version,
          "metadata": data.metadata
        });

        flux.actions.setResourceRunning("input", data.plan.name, false);
      },

      "output": function(data) {
        flux.actions.addResource("output", {
          "name": data.plan.name,
          "version": data.version,
          "metadata": data.metadata
        });

        flux.actions.setResourceRunning("output", data.plan.name, false);
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

function renderResource(event, type) {
  var resource = event[type];
  flux.actions.addResource(type, resource);
  flux.actions.setResourceRunning(type, resource.name, false);
}

function processLogs(event) {
  var log;

  switch(event.origin.type) {
  case "run":
    log = "run";
    break;
  case "input":
  case "output":
    flux.actions.setResourceRunning(event.origin.type, event.origin.name, true);
    log = event.origin.type + "-" + event.origin.name;
  }

  if(!log || !event.payload) {
    return;
  }

  flux.actions.addLog(log, event.payload);
}

function processError(event) {
  var log;

  if(event.origin && event.origin.type) {
    switch(event.origin.type) {
    case "input":
    case "output":
      flux.actions.setResourceRunning(event.origin.type, event.origin.name, false);
      flux.actions.setResourceErrored(event.origin.type, event.origin.name, true);
      log = event.origin.type + "-" + event.origin.name;
    }
  } else {
    log = 'run'
  }

  if(!log) {
    return;
  }
  flux.actions.addError(log, event.message);
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
window.renderResource = renderResource;
