var React = require('react/addons');
var ImmutableRenderMixin = require('react-immutable-render-mixin');

var Logs = require('./logs.jsx');
var Resource = require('./resource.jsx');

var Fluxxor = require('fluxxor');
var FluxMixin = Fluxxor.FluxMixin(React),
    StoreWatchMixin = Fluxxor.StoreWatchMixin;
var flux = require('./stores');

var ansiparse = require('./ansiparse');

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
    if (this.state.resources.has('input')) {
      this.state.resources.get('input').forEach(function(input) {
        var logs;
        var key = 'input-' + input.get('name');
        if (this.state.logs.has(key)) {
          logs = this.state.logs.get(key);
        }
        inputResources.push(<Resource key={key} resource={input} logs={logs} autoscroll={this.state.autoscroll} />);
      }, this);
    }
    var outputResources = [];
    if (this.state.resources.has('output')) {
      this.state.resources.get('output').forEach(function(output) {
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
  var eventHandler;
  var currentVersion;

  es.addEventListener("version", function(event) {
    successfullyConnected = true;

    if (eventHandler) {
      for (var key in eventHandler) {
        es.removeEventListener(key, eventHandler[key]);
      }
    }

    currentVersion = JSON.parse(event.data);
    eventHandler = eventHandlers[currentVersion];

    for (var key in eventHandler) {
      es.addEventListener(key, eventHandler[key], false);
    }
  });

  es.addEventListener("end", function(event) {
    es.close();
  });

  es.onerror = function(event) {
    if(currentVersion != "1.1") {
      // versions < 1.1 cannot distinguish between end of stream and an
      // interrupted connection
      es.close();
    }

    if(!successfullyConnected) {
      // assume rejected because of auth
      $("#build-requires-auth").show();
    }
  };
}

var moment = require('moment');
require("moment-duration-format");

var v1Handlers = {
  "log": function(msg) {
    processLogs(JSON.parse(msg.data));
  },

  "error": function(msg) {
    if (msg.data === undefined) {
      // 'error' event may also be native browser error, unfortunately
      return
    }

    processError(JSON.parse(msg.data));
  },

  "status": function(msg) {
    var event = JSON.parse(msg.data);

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
  },

  "input": function(msg) {
    renderResource(JSON.parse(msg.data), msg.type);
  },

  "output": function(msg) {
    renderResource(JSON.parse(msg.data), msg.type);
  }
}

var eventHandlers = {
  "1.0": v1Handlers,

  "1.1": v1Handlers,
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

  if(event.origin) {
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

