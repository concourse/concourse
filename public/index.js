function draw(groups, renderFn) {
  $.get("/api/v1/jobs", function(jobsPayload) {
    var jobs = JSON.parse(jobsPayload);

    $.get("/api/v1/resources", function(resourcesPayload) {
      var resources = JSON.parse(resourcesPayload);

      var graph = generateGraph(groups, jobs, resources);

      graph.nodes().forEach(function(v) {
        var node = graph.node(v);

        if (node.gateway) {
          node.height = 30;
          node.width = 2;
        }

        node.paddingLeft = 0;
        node.paddingRight = 0;
        node.paddingTop = 0;
        node.paddingBottom = 0;
      });

      graph.edges().forEach(function(e) {
        var edge = graph.edge(e);

        // curvy
        edge.lineInterpolate = "bundle";
        edge.lineTension = 1.0;
      });

      renderFn(graph);

      graph.edges().forEach(function(e) {
        var edge = graph.edge(e);

        if (edge.status) {
          $("#"+edge.id).attr("class", "path " + edge.status);
        }
      });
    });
  });
}

function drawContinuously(render, svg, groups) {
  draw(groups, function(graph) {
    render(svg, graph);

    svg.attr("viewBox", "-20 -20 " + (graph.graph().width + 40) + " " + (graph.graph().height + 40));

    setTimeout(function() {
      drawContinuously(render, svg, groups)
    }, 5000)
  });
}

function renderPipeline(groups) {
  var svg = d3.select("svg");

  svg.call(d3.behavior.zoom().on("zoom", function() {
    var ev = d3.event;
    svg.select("g.output")
       .attr("transform", "translate(" + ev.translate + ") scale(" + ev.scale + ")");
  }));

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

  drawContinuously(render, svg, groups);
}
