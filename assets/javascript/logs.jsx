var React = require('react/addons');
var ImmutableRenderMixin = require('react-immutable-render-mixin');

var LogLine = React.createClass({
  mixins: [ImmutableRenderMixin],

  componentDidMount: function() {
    if (this.props.autoscroll) {
      window.scrollTo(0, document.body.scrollHeight);
    }
  },

  renderSequence: function(sequence, key) {
    var classString = "";

    if (sequence.linebreak) {
      classString += "linebreak ";
    }
    if (sequence.foreground) {
      classString += "ansi-"+sequence.foreground+"-fg "; 
    }
    if (sequence.background) {
      classString += "ansi-"+sequence.background+"-bg "; 
    }
    if (sequence.bold) {
      classString += "ansi-bold ";
    }
    if (sequence.error) {
      classString += "error ";
    }

    return (
      <span key={key} className={classString}>{sequence.text}</span>
    )
  },

  render: function() {
    return (
      <div>{this.props.line.toJS().map(this.renderSequence)}</div>
    )
  },
});

var LogsLineBatch = React.createClass({
  mixins: [ImmutableRenderMixin],

  render: function() {
    var lines = [];
    if (this.props.lines) {
      this.props.lines.forEach(function(line, i) {
        lines.push(<LogLine key={i} line={line} autoscroll={this.props.autoscroll} />)
      }, this);
    }

    return (
      <div>{lines}</div>
    );
  },
});

var Logs = React.createClass({
  mixins: [ImmutableRenderMixin],

  render: function() {
    var batches = [];
    this.props.batches.forEach(function(lineBatch, i) {
      batches.push(<LogsLineBatch key={i} lines={lineBatch} autoscroll={this.props.autoscroll} />)
    }, this);

    return (
      <pre>{batches}</pre>
    );
  },
});

module.exports = Logs;
