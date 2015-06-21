function Graph() {
  this._nodes = {};
  this._edges = [];
};

Graph.prototype.setNode = function(id, value) {
  this._nodes[id] = value;
};

Graph.prototype.removeNode = function(id) {
  var node = this._nodes[id];

  for (var i in node._inEdges) {
    var edge = node._inEdges[i];
    var sourceNode = edge.source.node;
    var idx = sourceNode._outEdges.indexOf(edge);
    sourceNode._outEdges.splice(idx, 1);

    var graphIdx = this._edges.indexOf(edge);
    this._edges.splice(graphIdx, 1);
  }

  for (var i in node._outEdges) {
    var edge = node._outEdges[i];
    var targetNode = edge.target.node;
    var idx = targetNode._inEdges.indexOf(edge);
    targetNode._inEdges.splice(idx, 1);

    var graphIdx = this._edges.indexOf(edge);
    this._edges.splice(graphIdx, 1);
  }

  delete this._nodes[id];
}

Graph.prototype.addEdge = function(sourceId, targetId, key) {
  var source = this._nodes[sourceId];
  if (source === undefined) {
    throw "source node does not exist: " + sourceId;
  }

  var target = this._nodes[targetId];
  if (target === undefined) {
    throw "target node does not exist: " + targetId;
  }

  for (var i in target._inEdges) {
    if (target._inEdges[i].source.node.id == source.id) {
      // edge already exists; skip
      return;
    }
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
}

Graph.prototype.removeEdge = function(edge) {
  var inIdx = edge.target.node._inEdges.indexOf(edge);
  edge.target.node._inEdges.splice(inIdx, 1);

  var outIdx = edge.source.node._outEdges.indexOf(edge);
  edge.source.node._outEdges.splice(outIdx, 1);

  var graphIdx = this._edges.indexOf(edge);
  this._edges.splice(graphIdx, 1);
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
  var columns = [];

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
      } else if (targetA && targetB) {
        var introRankA = targetA.rankOfFirstAppearance();
        var introRankB = targetB.rankOfFirstAppearance();
        if(introRankA < introRankB) {
          return -1;
        } else if (introRankA > introRankB) {
          return 1;
        }
      }

      return compareNames(a, b);
    });
  }

  // run twice so that second pass can use positioning from first pass in
  // output-based comparisons, since we process the columns left-to-right
  for (var repeat = 0; repeat < 2; repeat++) {
    for (var i in columns) {
      columns[i].sortNodes();
      columns[i].layout();
    }
  }

  // add spacing between nodes to align with upstream/downstream;
  // walk the columns right-to-left
  for (var i = columns.length - 1; i >= 0; i--) {
    columns[i].pullDown()
  }
}

Graph.prototype.computeRanks = function() {
  var forwardNodes = {};

  for (var n in this._nodes) {
    var node = this._nodes[n];

    if (node._inEdges.length == 0) {
      node._cachedRank = 0;
      forwardNodes[node.id] = node;
    }
  }

  var bottomNodes = {};

  // walk over all nodes from left to right and determine their rank
  while (!objectIsEmpty(forwardNodes)) {
    var nextNodes = {};

    for (var n in forwardNodes) {
      var node = forwardNodes[n];

      if (node._outEdges.length == 0) {
        bottomNodes[node.id] = node;
      }

      for (var e in node._outEdges) {
        var nextNode = node._outEdges[e].target.node;

        // careful: two edges may go to the same node but be from different
        // ranks, so always destination nodes as far to the right as possible
        nextNode._cachedRank = Math.max(nextNode._cachedRank, node._cachedRank + 1);

        nextNodes[nextNode.id] = nextNode;
      }
    }

    forwardNodes = nextNodes;
  }

  var backwardNodes = bottomNodes;

  // walk over all nodes from right to left and bring upstream nodes as far
  // to the right as possible, so that edges aren't passing through ranks
  while (!objectIsEmpty(backwardNodes)) {
    var prevNodes = {};

    for (var n in backwardNodes) {
      var node = backwardNodes[n];

      // for all upstream nodes, determine rightmost possible column by taking
      // the minimum rank of all downstream nodes and placing it in the rank
      // immediately preceding it
      for (var e in node._inEdges) {
        var prevNode = node._inEdges[e].source.node;

        var rightmostRank = prevNode.rightmostPossibleRank();
        if (rightmostRank !== undefined) {
          prevNode._cachedRank = rightmostRank;
        }

        prevNodes[prevNode.id] = prevNode;
      }
    }

    backwardNodes = prevNodes;
  }
};

Graph.prototype.collapseEquivalentNodes = function() {
  var nodesByRank = [];

  for (var n in this._nodes) {
    var node = this._nodes[n];

    var byRank = nodesByRank[node.rank()];
    if (byRank === undefined) {
      byRank = {};
      nodesByRank[node.rank()] = byRank;
    }

    if (node.equivalentBy === undefined) {
      continue;
    }

    byEqv = byRank[node.equivalentBy];
    if (byEqv === undefined) {
      byEqv = [];
      byRank[node.equivalentBy] = byEqv;
    }

    byEqv.push(node);
  }

  for (var r in nodesByRank) {
    var byEqv = nodesByRank[r];
    for (var e in byEqv) {
      var nodes = byEqv[e];
      if (nodes.length == 1) {
        continue;
      }

      var chosenOne = nodes[0];
      for (var i = 1; i < nodes.length; i++) {
        var loser = nodes[i];

        for (var ie in loser._inEdges) {
          var edge = loser._inEdges[ie];
          this.addEdge(edge.source.node.id, chosenOne.id, edge.key);
        }

        for (var oe in loser._outEdges) {
          var edge = loser._outEdges[oe];
          this.addEdge(chosenOne.id, edge.target.node.id, edge.key);
        }

        this.removeNode(loser.id);
      }
    }
  }
}

Graph.prototype.addSpacingNodes = function() {
  var edgesToRemove = [];
  for (var e in this._edges) {
    var edge = this._edges[e];
    var delta = edge.target.node.rank() - edge.source.node.rank();
    if (delta > 1) {
      var upstreamNode = edge.source.node;

      for (var i = 0; i < (delta - 1); i++) {
        var spacerID = edge.source.node.id + "-spacing-" + i;

        var spacingNode = this.node(spacerID);
        if (!spacingNode) {
          spacingNode = upstreamNode.copy();
          spacingNode.id = spacerID;
          spacingNode._cachedRank = upstreamNode.rank() + 1;
          this.setNode(spacingNode.id, spacingNode);
        }

        this.addEdge(upstreamNode.id, spacingNode.id, edge.key);

        upstreamNode = spacingNode;
      }

      this.addEdge(upstreamNode.id, edge.target.node.id, edge.key);

      edgesToRemove.push(edge);
    }
  }

  for (var e in edgesToRemove) {
    this.removeEdge(edgesToRemove[e]);
  }
}

function Column(idx) {
  this.index = idx;
  this.nodes = [];

  this._spacing = 10;
}

Column.prototype.pullDown = function() {
  for (var nodeIdx = 0; nodeIdx < this.nodes.length; nodeIdx++) {
    var node = this.nodes[nodeIdx];

    var delta = 0;

    var downstreamY = node.deltaToHighestDownstreamTarget();
    if (downstreamY !== undefined && downstreamY > 0) {
      delta = downstreamY;
    }

    var upstreamY = node.deltaToHighestUpstreamSource();
    if (upstreamY !== undefined && upstreamY > 0) {
      delta = upstreamY;
    }

    if (delta == 0) {
      continue;
    }

    for (var i = nodeIdx; i < this.nodes.length; i++) {
      this.nodes[i]._position.y += delta;
    }
  }
}

Column.prototype.sortNodes = function() {
  var nodes = this.nodes;

  nodes.sort(function(a, b) {
    if (a._inEdges.length && b._inEdges.length) {
      // position nodes closer to their upstream sources
      var byHighestSource = a.highestUpstreamSource() - b.highestUpstreamSource();
      if (byHighestSource != 0) {
        return byHighestSource;
      }
    }

    if (a._outEdges.length && b._outEdges.length) {
      // position nodes closer to their downstream targets
      var byHighestTarget = a.highestDownstreamTarget() - b.highestDownstreamTarget();
      if (byHighestTarget != 0) {
        return byHighestTarget;
      }
    }

    if (a._inEdges.length && b._outEdges.length) {
      // position nodes closer to their upstream sources or downstream targets
      var compare = a.highestUpstreamSource() - b.highestDownstreamTarget();
      if (compare != 0) {
        return compare;
      }
    }

    if (a._outEdges.length && b._inEdges.length) {
      // position nodes closer to their upstream sources or downstream targets
      var compare = a.highestDownstreamTarget() - b.highestUpstreamSource();
      if (compare != 0) {
        return compare;
      }
    }

    // place nodes that threaded through upstream nodes higher
    var aPassedThrough = a.passedThroughAnyPreviousNode();
    var bPassedThrough = b.passedThroughAnyPreviousNode();
    if (aPassedThrough && !bPassedThrough) {
      return -1;
    }

    // place nodes that thread through downstream nodes higher
    var aPassesThrough = a.passesThroughAnyNextNode();
    var bPassesThrough = b.passesThroughAnyNextNode();
    if (aPassesThrough && !bPassesThrough) {
      return -1;
    }

    // place nodes with more out edges higher
    var byOutEdges = b._outEdges.length - a._outEdges.length;
    if (byOutEdges != 0) {
      return byOutEdges;
    }

    if (!aPassesThrough && bPassesThrough) {
      return 1;
    }

    // both are of equivalent; compare names so it's at least deterministic

    a.debugMarked = true; // to aid in debugging (adds .marked css class)
    b.debugMarked = true;

    return compareNames(a.name, b.name);
  });
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

function Node(opts) {
  // Graph node ID
  this.id = opts.id;
  this.name = opts.name;
  this.class = opts.class;
  this.status = opts.status;
  this.key = opts.key;
  this.url = opts.url;
  this.svg = opts.svg;
  this.equivalentBy = opts.equivalentBy;

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

Node.prototype.copy = function() {
  return new Node({
    id: this.id,
    name: this.name,
    class: this.class,
    status: this.status,
    key: this.key,
    url: this.url,
    svg: this.svg,
    equivalentBy: this.equivalentBy
  });
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

Node.prototype.column = function() {
  return this.rank();
};

Node.prototype.rank = function() {
  return this._cachedRank;
}

Node.prototype.rightmostPossibleRank = function() {
  var rightmostRank;

  for (var o in this._outEdges) {
    var prevTargetNode = this._outEdges[o].target.node;
    var targetPrecedingRank = prevTargetNode.rank() - 1;

    if (rightmostRank === undefined) {
      rightmostRank = targetPrecedingRank;
    } else {
      rightmostRank = Math.min(rightmostRank, targetPrecedingRank);
    }
  }

  return rightmostRank;
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

Node.prototype.highestUpstreamSource = function() {
  var minY;

  var y;
  for (var e in this._inEdges) {
    y = this._inEdges[e].source.position().y;

    if (minY === undefined || y < minY) {
      minY = y;
    }
  }

  return minY;
};

Node.prototype.highestDownstreamTarget = function() {
  var minY;

  var y;
  for (var e in this._outEdges) {
    y = this._outEdges[e].target.position().y;

    if (minY === undefined || y < minY) {
      minY = y;
    }
  }

  return minY;
};

Node.prototype.deltaToHighestUpstreamSource = function() {
  var minY;
  var highestUpstream;

  var y;
  for (var e in this._inEdges) {
    var edge = this._inEdges[e];
    y = edge.source.position().y;

    if (minY === undefined || y < minY) {
      minY = y;
      highestUpstream = edge;
    }
  }

  if (highestUpstream) {
    return highestUpstream.source.position().y - highestUpstream.target.position().y;
  }
};

Node.prototype.deltaToHighestDownstreamTarget = function() {
  var minY;
  var highestDownstream;

  var y;
  for (var e in this._outEdges) {
    var edge = this._outEdges[e];
    y = edge.target.position().y;

    if (minY === undefined || y < minY) {
      minY = y;
      highestDownstream = edge;
    }
  }

  if (highestDownstream) {
    return highestDownstream.target.position().y - highestDownstream.source.position().y;
  }
};

Node.prototype.passedThroughAnyPreviousNode = function() {
  for (var e in this._inEdges) {
    var edge = this._inEdges[e];
    if (edge.key in edge.source.node._edgeTargets) {
      return true;
    }
  }

  return false;
};

Node.prototype.passesThroughAnyNextNode = function() {
  for (var e in this._outEdges) {
    var edge = this._outEdges[e];
    if (edge.key in edge.target.node._edgeSources) {
      return true;
    }
  }

  return false;
};

function Edge(source, target, key) {
  this.source = source;
  this.target = target;
  this.key = key;
}

Edge.prototype.id = function() {
  return this.source.id() + "-to-" + this.target.id();
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

EdgeTarget.prototype.rankOfFirstAppearance = function() {
  var inEdges = this.node._inEdges;
  var minRank = Infinity;
  for (var i in inEdges) {
    var inEdge = inEdges[i];

    if (inEdge.source.key == this.key) {
      var upstreamNodeInEdges = inEdge.source.node._inEdges;

      if (upstreamNodeInEdges.length == 0) {
        return inEdge.source.node.rank();
      }

      var foundUpstreamInEdge = false;
      for (var j in upstreamNodeInEdges) {
        var upstreamEdge = upstreamNodeInEdges[j];

        if (upstreamEdge.target.key == this.key) {
          foundUpstreamInEdge = true;

          var rank = upstreamEdge.target.rankOfFirstAppearance()

          if (rank < minRank) {
            minRank = rank;
          }
        }
      }

      if (!foundUpstreamInEdge) {
        return inEdge.source.node.rank();
      }
    }
  }

  return minRank;
}

EdgeTarget.prototype.id = function() {
  return this.node.id + "-" + this.key + "-target";
}

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

function compareNames(a, b) {
  var byLength = a.length - b.length;
  if (byLength != 0) {
    // place shorter names higher. pretty arbitrary but looks better.
    return byLength;
  }

  return a.localeCompare(b);
}
