var React = require('react/addons');
var Immutable = require('immutable');
var ImmutableRenderMixin = require('react-immutable-render-mixin');
var FluxMixin = require('fluxxor').FluxMixin(React);
var Logs = require('./logs.jsx');

var Step = React.createClass({
  mixins: [FluxMixin, ImmutableRenderMixin],

  toggleLogs: function() {
    var model = this.props.model;
    this.getFlux().actions.toggleStepLogs(model.origin());
  },

  render: function() {
    var model = this.props.model;

    var versionDetails = [];
    var version = model.version();
    if (version !== undefined) {
      for (var key in version) {
        var value = version[key];
        versionDetails.push(<dt key={"version-dt-"+key}>{key}</dt>);
        versionDetails.push(<dd key={"version-dd-"+key}>{value}</dd>);
      }
    }

    var metadataDetails = [];
    var metadata = model.metadata();
    if (metadata !== undefined) {
      metadata.forEach(function(field) {
        metadataDetails.push(<dt key={"metadata-dt-"+field.name}>{field.name}</dt>);
        metadataDetails.push(<dd key={"metadata-dd-"+field.name}>{field.value}</dd>);
      });
    }

    var cx = React.addons.classSet;
    var classNames = cx({
      "build-step": true,
      "running": model.isRunning(),
      "errored": model.isErrored(),
      "first-occurrence": model.isFirstOccurrence()
    });

    var displayLogs = model.isShowingLogs() ? 'block' : 'none';

    return (
      <div className={classNames}>
        <div className="header" onClick={this.toggleLogs}>
          <h3>{model.origin().name}</h3>

          <dl className="version">{versionDetails}</dl>

          <div style={{clear: 'both'}}></div>
        </div>

        <div className="resource-body" style={{display: displayLogs}}>
          <dl className="build-metadata">{metadataDetails}</dl>

          <Logs batches={this.props.logs} autoscroll={this.props.autoscroll} />

          <div style={{clear: 'both'}}></div>
        </div>
      </div>
    )
  },
});

module.exports = Step;
