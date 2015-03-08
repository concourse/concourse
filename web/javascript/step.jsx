var React = require('react/addons');
var Immutable = require('immutable');
var ImmutableRenderMixin = require('react-immutable-render-mixin');
var FluxMixin = require('fluxxor').FluxMixin(React);
var Logs = require('./logs.jsx');

var Step = React.createClass({
  mixins: [FluxMixin, ImmutableRenderMixin],

  toggleLogs: function() {
    var model = this.props.model;
    this.getFlux().actions.toggleStepLogs(model.origin);
  },

  render: function() {
    var model = this.props.model;

    var versionDetails = [];
    if (model.version !== undefined) {
      for (var key in model.version) {
        var value = model.version[key];
        versionDetails.push(<dt key={"version-dt-"+key}>{key}</dt>);
        versionDetails.push(<dd key={"version-dd-"+key}>{value}</dd>);
      }
    }

    var metadataDetails = [];
    if (model.metadata !== undefined) {
      model.metadata.forEach(function(metadata) {
        metadataDetails.push(<dt key={"metadata-dt-"+metadata.name}>{metadata.name}</dt>);
        metadataDetails.push(<dd key={"metadata-dd-"+metadata.name}>{metadata.value}</dd>);
      });
    }

    var cx = React.addons.classSet;
    var classNames = cx({
      "build-step": true,
      "running": model.running,
      "errored": model.errored,
      "first-occurrence": model.first_occurrence
    });

    var displayLogs = model.showLogs ? 'block' : 'none';

    return (
      <div className={classNames}>
        <div className="header" onClick={this.toggleLogs}>
          <h3>{model.origin.name}</h3>

          <dl className="version">{versionDetails}</dl>

          <div style={{clear: 'both'}}></div>
        </div>

        <div className="resource-body" style={{display: displayLogs}}>
          <dl className="build-metadata">{metadataDetails}</dl>

          <Logs batches={this.props.logs || Immutable.List()} autoscroll={this.props.autoscroll} />

          <div style={{clear: 'both'}}></div>
        </div>
      </div>
    )
  },
});

module.exports = Step;
