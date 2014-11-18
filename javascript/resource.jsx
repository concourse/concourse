var React = require('react/addons');
var Immutable = require('immutable');
var ImmutableRenderMixin = require('react-immutable-render-mixin');
var FluxMixin = require('fluxxor').FluxMixin(React);
var Logs = require('./logs.jsx');

var Resource = React.createClass({
  mixins: [FluxMixin, ImmutableRenderMixin],

  toggleLogs: function() {
    var resource = this.props.resource;
    this.getFlux().actions.toggleResourceLogs(resource.get('kind'), resource.get('name'));
  },

  render: function() {
    var resource = this.props.resource;
    var versionDetails = [];
    resource.get('version').forEach(function(version, key) {
      versionDetails.push(<dt key={"version-dt-"+key}>{key}</dt>);
      versionDetails.push(<dd key={"version-dd-"+key}>{version}</dd>);
    });
    var metadataDetails = [];
    resource.get('metadata').forEach(function(metadata) {
      var metadata = metadata.toJS();
      metadataDetails.push(<dt key={"metadata-dt-"+metadata.name}>{metadata.name}</dt>);
      metadataDetails.push(<dd key={"metadata-dd-"+metadata.name}>{metadata.value}</dd>);
    });

    var cx = React.addons.classSet;
    var classNames = cx({
      "build-source": true,
      "running": resource.get('running'),
      "errored": resource.get('errored'),
      "first-occurrence": resource.get('first_occurence'),
    });

    var displayLogs = resource.get('showLogs') ? 'block' : 'none';

    return (
      <div className={classNames}>
        <div className="header" onClick={this.toggleLogs}>
          <h3>{resource.get('name')}</h3>

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

module.exports = Resource;
