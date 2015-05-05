function Graph() {
  this._nodes = {};
  this._edges = [];
};

Graph.prototype.setNode = function(id, value) {
  this._nodes[id] = value;
};

Graph.prototype.addEdge = function(sourceId, targetId, key) {
  var source = this._nodes[sourceId];
  if (source === undefined) {
    throw "source node does not exist: " + sourceId;
  }

  var target = this._nodes[targetId];
  if (target === undefined) {
    throw "target node does not exist: " + targetId;
  }

  if (source._edgeKeys.indexOf(key) == -1) {
    source._edgeKeys.push(key);
  }

  if (target._edgeKeys.indexOf(key) == -1) {
    target._edgeKeys.push(key);
  }

  var edgeSource = source._edgeSources[key];
  if (!edgeSource) {
    edgeSource = new EdgeSource(source, key);
    source._edgeSources[key] = edgeSource;
  }

  var edgeTarget = target._edgeTargets[key];
  if (!edgeTarget) {
    edgeTarget = new EdgeTarget(target, key);
    target._edgeTargets[key] = edgeTarget;
  }

  var edge = new Edge(edgeSource, edgeTarget, key);
  target._inEdges.push(edge);
  source._outEdges.push(edge);
  this._edges.push(edge);

  source._clearCaches();
  target._clearCaches();
}

Graph.prototype.node = function(id) {
  return this._nodes[id];
};

Graph.prototype.nodes = function() {
  var nodes = [];

  for (var id in this._nodes) {
    nodes.push(this._nodes[id]);
  }

  return nodes;
};

Graph.prototype.edges = function() {
  return this._edges;
};

Graph.prototype.layout = function() {
  var columns = {};

  for (var i in this._nodes) {
    var node = this._nodes[i];

    var columnIdx = node.column();
    var column = columns[columnIdx];
    if (!column) {
      column = new Column(columnIdx);
      columns[columnIdx] = column;
    }

    column.nodes.push(node);
  }

  for (var c in columns)
    columns[c]._cacheEdges()

  for (var i in this._nodes) {
    var node = this._nodes[i];

    var column = node.column();

    var columnOffset = 0;
    for (var c in columns) {
      if (c < column) {
        columnOffset += columns[c].width() + 50;
      }
    }

    node._position.x = columnOffset + ((columns[column].width() - node.width()) / 2);

    node._edgeKeys.sort(function(a, b) {
      var targetA = node._edgeTargets[a];
      var targetB = node._edgeTargets[b];

      if (targetA && !targetB) {
        return -1;
      } else if (!targetA && targetB) {
        return 1;
      }

      var aIsConnected = targetA && targetA.isConnected();
      var bIsConnected = targetB && targetB.isConnected();

      if (aIsConnected && !bIsConnected) {
        return -1;
      } else if (!aIsConnected && bIsConnected) {
        return 1;
      }

      return a.localeCompare(b);
    });
  }

  for (var c in columns) {
    columns[c].layout();
  }

  for (var i = 0; i < 10; i++) {
    for (var c in columns) {
      columns[c].improve();
    }
  }
}

function Column(idx) {
  this.index = idx;
  this.nodes = [];

  this._spacing = 10;
}

Column.prototype.improve = function() {
  var nodes = this.nodes;

  var beforeCrossing = this.crossingLines();
  var beforeStraight = this.straightLines();
  var beforeCost = this.cost();

  for (var i = nodes.length-1; i >= 0; i--) {
    var nodeIdx = i;

    for (var j = 0; j < nodes.length; j++) {
      if (nodeIdx == j) {
        continue;
      }

      var before = beforeCrossing.inputs + beforeCrossing.outputs;

      this.swap(nodeIdx, j)

      var afterCrossing = this.crossingLines();
      var afterStraight = this.straightLines();
      var afterCost = this.cost();

      var after = afterCrossing.inputs + afterCrossing.outputs;

      if (
        // fewer crossing overall
        after < before ||

        // same crossing but fewer crossing inputs (next column may fix outputs)
        (after == before && afterCrossing.inputs < beforeCrossing.inputs) ||

        // same crossing but nodes are closer
        (after == before && afterCost < beforeCost) ||

        // same crossing but more lines are straight
        (after == before && afterStraight > beforeStraight)
      ) {
        nodeIdx = j;

        beforeCrossing = afterCrossing;
        beforeStraight = afterStraight;
        beforeCost = afterCost;
      } else {
        this.swap(nodeIdx, j)
      }
    }
  }
}

Column.prototype.swap = function(a, b) {
  var tmp = this.nodes[a];
  this.nodes[a] = this.nodes[b];
  this.nodes[b] = tmp;

  this.layout();
}

Column.prototype.cost = function() {
  var cost = 0;

  var nodes = this.nodes,
      totalNodes = nodes.length

  for (var i = 0; i < totalNodes; i++) {
    cost += nodes[i].travel();
  }

  return cost;
}

Column.prototype.width = function() {
  var width = 0;

  for (var i in this.nodes) {
    width = Math.max(width, this.nodes[i].width())
  }

  return width;
}

Column.prototype.layout = function() {
  var rollingOffset = 0;

  for (var i in this.nodes) {
    var node = this.nodes[i];

    node._position.y = rollingOffset;

    rollingOffset += node.height() + this._spacing;
  }
}

function crossingEdges(edges) {
  var crossingLines = 0;

  var totalEdges = edges.length;
  for (var i = 0; i < totalEdges; i++) {
    var edgeA = edges[i];
    var edgeASourceY = edgeA.source.y();
    var edgeATargetY = edgeA.target.y();

    for (var j = 0; j < totalEdges; j++) {
      var edgeB = edges[j];
      var edgeBSourceY = edgeB.source.y();
      var edgeBTargetY = edgeB.target.y();

      if (edgesAreCrossing(edgeASourceY, edgeATargetY, edgeBSourceY, edgeBTargetY)) {
        crossingLines++;
      }
    }
  }

  return crossingLines;
}

function edgesAreCrossing(edgeASourceY, edgeATargetY, edgeBSourceY, edgeBTargetY) {
  return (edgeASourceY < edgeBSourceY && edgeATargetY > edgeBTargetY) ||
         (edgeASourceY > edgeBSourceY && edgeATargetY < edgeBTargetY)
}

Column.prototype.crossingLines = function() {
  return {
    inputs: crossingEdges(this._allInEdges),
    outputs: crossingEdges(this._allOutEdges)
  }
}

Column.prototype.straightLines = function() {
  var straightLines = 0;

  var nodes = this.nodes,
      totalNodes = nodes.length;

  for (var i = 0; i < totalNodes; i++) {
    straightLines += nodes[i].straightLines();
  }

  return straightLines;
}

Column.prototype._cacheEdges = function() {
  this._allInEdges = [];
  this._allOutEdges = [];

  var nodes = this.nodes;
  var totalNodes = this.nodes.length;
  for (var i = 0; i < totalNodes; i++) {
    this._allInEdges = this._allInEdges.concat(nodes[i]._inEdges);
    this._allOutEdges = this._allOutEdges.concat(nodes[i]._outEdges);
  }
}

function Node(opts) {
  // Graph node ID
  this.id = opts.id;
  this.name = opts.name;
  this.class = opts.class;
  this.status = opts.status;
  this.key = opts.key;
  this.url = opts.url;
  this.svg = opts.svg;

  // DOM element
  this.label = undefined;

  // [EdgeTarget]
  this._edgeTargets = {};

  // [EdgeSource]
  this._edgeSources = {};

  this._edgeKeys = [];
  this._inEdges = [];
  this._outEdges = [];

  this._cachedRank = -1;
  this._cachedWidth = 0;

  // position (determined by graph.layout())
  this._position = {
    x: 0,
    y: 0
  };
};

Node.prototype.width = function() {
  if (this._cachedWidth == 0) {
    var id = this.id;

    var svgNode = this.svg.selectAll("g.node").filter(function(node) {
      return node.id == id;
    })

    var textNode = svgNode.select("text").node();

    if (textNode) {
      this._cachedWidth = textNode.getBBox().width;
    } else {
      return 0;
    }
  }

  return this._cachedWidth + 10;
}

Node.prototype.height = function() {
  var keys = Math.max(this._edgeKeys.length, 1);
  return (20 * keys) + (10 * (keys - 1));
}

Node.prototype.position = function() {
  return this._position;
}

Node.prototype.travel = function() {
  var travel = 0;

  var inEdges = this._inEdges,
      totalInEdges = inEdges.length;

  var outEdges = this._outEdges,
      totalOutEdges = outEdges.length;

  for (var i = 0; i < totalInEdges; i++) {
    travel += Math.abs(inEdges[i].dy());
  }

  for (var i = 0; i < totalOutEdges; i++) {
    travel += Math.abs(outEdges[i].dy());
  }

  return travel;
}

Node.prototype.straightLines = function() {
  var straightLines = 0;

  var inEdges = this._inEdges,
      totalInEdges = inEdges.length;

  var outEdges = this._outEdges,
      totalOutEdges = outEdges.length;

  for (var i = 0; i < totalInEdges; i++) {
    if (inEdges[i].dy() == 0) {
      straightLines += 1;
    }
  }

  for (var i = 0; i < totalOutEdges; i++) {
    if (outEdges[i].dy() == 0) {
      straightLines += 1;
    }
  }

  return straightLines;
}

Node.prototype.column = function() {
  if (this._inEdges.length == 0 || this.dependsOn(this, [])) {
    var nextmostRank = Infinity;

    for (var i in this._outEdges) {
      nextmostRank = Math.min(nextmostRank, this._outEdges[i].target.node.rank() - 1)
    }

    if (nextmostRank == Infinity) {
      return 0;
    }

    return nextmostRank;
  }

  return this.rank();
};

Node.prototype.rank = function() {
  if (this._cachedRank != -1) {
    return this._cachedRank;
  }

  var rank = -1;

  for (var i in this._inEdges) {
    var source = this._inEdges[i].source.node;

    if (source.dependsOn(this, [])) {
      continue;
    }

    rank = Math.max(rank, source.rank())
  }

  rank = rank + 1;

  this._cachedRank = rank;

  return rank;
}

Node.prototype.dependsOn = function(node, stack) {
  for (var i in this._inEdges) {
    var source = this._inEdges[i].source.node;

    if (source == node) {
      return true;
    }

    if (stack.indexOf(this) != -1) {
      continue;
    }

    stack.push(this)

    if (source.dependsOn(node, stack)) {
      return true;
    }
  }

  return false;
}

Node.prototype._clearCaches = function() {
  this._cachedRank = -1;
  this._cachedPosition = undefined;
}

function Edge(source, target, key) {
  this.source = source;
  this.target = target;
  this.key = key;
}

Edge.prototype.id = function() {
  return this.source.id() + "-to-" + this.target.id();
}

Edge.prototype.dy = function() {
  return this.source.y() - this.target.y();
}

Edge.prototype.path = function() {
  var sourcePosition = this.source.position();
  var targetPosition = this.target.position();

  var curvature = 0.5;

  var x0 = sourcePosition.x,
      x1 = targetPosition.x,
      y0 = sourcePosition.y,
      y1 = targetPosition.y;

  var intermediatePoints = [];

  if (sourcePosition.x > targetPosition.x) {
    var belowSourceNode = this.source.node.position().y + this.source.node.height(),
        belowTargetNode = this.target.node.position().y + this.target.node.height();

    intermediatePoints = [
      (sourcePosition.x + 100) + "," + (belowSourceNode + 100),
      (targetPosition.x - 100) + "," + (belowTargetNode + 100),
    ]
  } else {
    var xi = d3.interpolateNumber(x0, x1),
        x2 = xi(curvature),
        x3 = xi(1 - curvature),

    intermediatePoints = [x2+","+y0, x3+","+y1]
  }

  return "M" + x0 + "," + y0 +" "
       + "C" + intermediatePoints.join(" ")
       + " " + x1 + "," + y1;
}

function EdgeSource(node, key) {
  // spacing between edge sources
  this._spacing = 30;

  // Node
  this.node = node;

  // Key
  this.key = key;
};

EdgeSource.prototype.width = function() {
  return 0;
}

EdgeSource.prototype.height = function() {
  return 0;
}

EdgeSource.prototype.id = function() {
  return this.node.id + "-" + this.key + "-source";
}

EdgeSource.prototype.position = function() {
  return {
    x: this.node.position().x + this.node.width(),
    y: this.y()
  }
};

EdgeSource.prototype.y = function() {
  var nodePosition = this.node.position();
  var index = this.node._edgeKeys.indexOf(this.key);
  return nodePosition.y + 10 + ((this.height() + this._spacing) * index)
}

function EdgeTarget(node, key) {
  // spacing between edge targets
  this._spacing = 30;

  // Node
  this.node = node;

  // Key
  this.key = key;
};

EdgeTarget.prototype.width = function() {
  return 0;
}

EdgeTarget.prototype.height = function() {
  return 0;
}

EdgeTarget.prototype.id = function() {
  return this.node.id + "-" + this.key + "-target";
}

EdgeTarget.prototype.isConnected = function() {
  var edges = this.node._inEdges;

  for (var i in edges) {
    if (edges[i].source.key == this.key) {
      if (edges[i].source.node._inEdges.length > 0) {
        return true;
      }
    }
  }

  return false;
};

EdgeTarget.prototype.position = function() {
  return {
    x: this.node.position().x,
    y: this.y()
  }
};

EdgeTarget.prototype.y = function() {
  var nodePosition = this.node.position();
  var index = this.node._edgeKeys.indexOf(this.key);

  return nodePosition.y + 10 + ((this.height() + this._spacing) * index)
}
