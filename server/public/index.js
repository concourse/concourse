function draw(nodes, edges) {
  var renderer = new dagreD3.Renderer();

  var oldDrawNodes = renderer.drawNodes();
  var oldDrawEdgePaths = renderer.drawEdgePaths();

  renderer.drawEdgePaths(function(graph, root) {
    var svgEdges = oldDrawEdgePaths(graph, root);

    svgEdges.attr("id", function(u) {
      return "edge-" + u;
    });

    graph.eachEdge(function(u) {
      var edge = graph.edge(u);

      if(edge.status) {
        $("#edge-"+u).attr("class", $("#edge-"+u).attr("class") + " " + edge.status);

        if (graph.isDirected() && root.select('#arrowhead-'+edge.status).empty()) {
          root
            .append('svg:defs')
              .append('svg:marker')
                .attr('id', 'arrowhead-'+edge.status)
                .attr('viewBox', '0 0 10 10')
                .attr('refX', 8)
                .attr('refY', 5)
                .attr('markerUnits', 'strokewidth')
                .attr('markerWidth', 8)
                .attr('markerHeight', 5)
                .attr('orient', 'auto')
                .append('svg:path')
                  .attr('d', 'M 0 0 L 10 5 L 0 10 z');
        }

        $("#edge-"+u+" path").attr("marker-end", "url(#arrowhead-"+edge.status+")");
      }
    });

    return svgEdges;
  });

  renderer.drawNodes(function(graph, root) {
    var svgNodes = oldDrawNodes(graph, root);

    svgNodes.attr("id", function(u) {
      return "node-" + u;
    });

    graph.eachNode(function(u) {
      var node = graph.node(u);

      if(node.type == "job") {
        $("#node-"+u).attr("class", $("#node-"+u).attr("class") + " job " + node.status);
      }

      $("#node-"+u+" rect").attr("rx", "0").attr("ry", "0");
    });

    return svgNodes;
  });

  var layout = renderer.layout(
    dagreD3.layout().rankDir("LR")).run(
      dagreD3.json.decode(nodes, edges),
      d3.select("svg g")
  );

  var svg = d3.select("svg")

  svg.attr("width", "100%");
  svg.attr("height", "100%");
  svg.attr("viewBox", "-20 -20 " + (layout.graph().width + 40) + " " + (layout.graph().height + 40));

  svg.call(d3.behavior.zoom().on("zoom", function() {
    var ev = d3.event;
    svg.select("g")
       .attr("transform", "translate(" + ev.translate + ") scale(" + ev.scale + ")");
  }));
}
