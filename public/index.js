function objectIsEmpty(obj) {
  for(var x in obj)
    return false;

  return true;
}

function nodeIsInGroups(groups, value) {
  if (!value.groups) {
    return false;
  }

  for(var i in value.groups) {
    if (groups[value.groups[i]]) {
      return true;
    }
  }

  return false;
}

function groupIntoSubgraphs(digraph) {
  var groupedGraph = new dagreD3.graphlib.Graph({ compound: true }).setGraph({
    rankDir: "LR"
  });

  digraph.nodes().forEach(function(v) {
    var node = digraph.node(v);
    groupedGraph.setNode(v, node);

    if (!node.groups || node.groups.length != 1) {
      return;
    }

    var group = node.groups[0];
    groupedGraph.setNode(group, {});
    groupedGraph.setParent(v, group);
  });

  digraph.edges().forEach(function(e) {
    var edge = digraph.edge(e);
    groupedGraph.setEdge(e.v, e.w, edge);
  });

  groupedGraph.nodes().forEach(function(v) {
    var commonGroup;

    var outE = groupedGraph.outEdges(v);
    for(var o in outE) {
      var edge = outE[o];
      var parent = groupedGraph.parent(edge.w);

      if (!commonGroup) {
        commonGroup = parent;
      }

      if (commonGroup != parent) {
        return
      }
    }

    var inE = groupedGraph.inEdges(v);
    for(var i in inE) {
      var edge = inE[i];
      var parent = groupedGraph.parent(edge.v);

      if (!commonGroup) {
        commonGroup = parent;
      }

      if (commonGroup != parent) {
        return
      }
    }

    if (commonGroup) {
      groupedGraph.setParent(v, commonGroup);
    }
  });

  return groupedGraph;
}

function removeOrphanedNodes(digraph) {
  digraph.nodes().forEach(function(v) {
    if (dagreD3.util.isSubgraph(digraph, v)) {
      return;
    }

    if (digraph.parent(v)) {
      return;
    }

    if (digraph.nodeEdges(v).length == 0) {
      digraph.removeNode(v);
    }
  });
}

function removeUnconnectedGroupMembers(groups, digraph) {
  for (var group in groups) {
    var enabled = groups[group];
    if (!enabled) {
      digraph.children(group).forEach(function(v) {
        var outE = digraph.outEdges(v);
        for(var o in outE) {
          var edge = outE[o];

          var targetValue = digraph.node(edge.w);
          if (nodeIsInGroups(groups, targetValue)) {
            return;
          }
        }

        var inE = digraph.inEdges(v);
        for(var i in inE) {
          var edge = inE[i];

          var sourceValue = digraph.node(edge.v);
          if (nodeIsInGroups(groups, sourceValue)) {
            return;
          }
        }

        digraph.removeNode(v);
      });
    }
  }
}

function computeGraph(groups, nodes, edges) {
  var digraph = dagreD3.graphlib.json.read({
    nodes: nodes,
    edges: edges,
    value: {
      rankDir: "LR"
    }
  });

  digraph.nodes().forEach(function(v) {
    var node = digraph.node(v);

    if (node.gateway) {
      node.height = 30;
      node.width = 2;
    }

    node.paddingLeft = 0;
    node.paddingRight = 0;
    node.paddingTop = 0;
    node.paddingBottom = 0;
  });

  if (!objectIsEmpty(groups)) {
    digraph = groupIntoSubgraphs(digraph);
    removeUnconnectedGroupMembers(groups, digraph);
    removeOrphanedNodes(digraph);
  }

  return digraph;
}

function draw(groups, nodes, edges) {
  var graph = computeGraph(groups, nodes, edges);

  var render = new dagreD3.render();

  render.arrows().status = function(parent, id, edge, type) {
    parent.append("svg:marker")
      .attr("id", id)
      .attr("class", "arrowhead-"+edge.status)
      .attr("viewBox", "0 0 10 10")
      .attr("refX", 8)
      .attr("refY", 5)
      .attr("markerWidth", 8)
      .attr("markerHeight", 5)
      .attr("orient", "auto")
      .append("svg:path")
      .attr("d", "M 0 0 L 10 5 L 0 10 z");
  };

  var svg = d3.select("svg");

  graph.edges().forEach(function(e) {
    var edge = graph.edge(e);

    // curvy
    edge.lineInterpolate = "bundle";
    edge.lineTension = 1.0;
  });

  render(svg, graph);

  graph.edges().forEach(function(e) {
    var edge = graph.edge(e);

    if (edge.status) {
      var edgeEle = $("#"+edge.id);

      // .addClass() does not work.
      edgeEle.attr("class", edgeEle.attr("class") + " " + edge.status);
    }
  });

  svg.attr("viewBox", "-20 -20 " + (graph.graph().width + 40) + " " + (graph.graph().height + 40));

  svg.call(d3.behavior.zoom().on("zoom", function() {
    var ev = d3.event;
    svg.select("g.output")
       .attr("transform", "translate(" + ev.translate + ") scale(" + ev.scale + ")");
  }));
}
