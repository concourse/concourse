import "./d3.v7.min.js";
import { Graphviz } from "./graphviz.min.js";

export async function renderCausality(dot){
  const foundSvg = d3.select(".causality-graph");
  const svg = createSvg(foundSvg);

  const graphviz = await Graphviz.load();
  svg.html(graphviz.layout(dot, "svg", "dot"));

  // disable tooltips that graphviz auto-generates
  svg.selectAll('title').remove();
  svg.selectAll('*').attr('xlink:title', null);

  var bbox = svg.node().getBBox()
  d3.select(svg.node().parentNode)
    .attr("viewBox", "" + (bbox.x - 20) + " " + (bbox.y - 20) + " " + (bbox.width + 40) + " " + (bbox.height + 40))
}

function createSvg(svg) {
  var g = d3.select("g.test")

  if (g.empty()) {
    svg.append("defs").append("filter")
      .attr("id", "embiggen")
      .append("feMorphology")
      .attr("operator", "dilate")
      .attr("radius", "4");

    g = svg.append("g").attr("class", "test")
    svg.on("mousedown", function(event) {
      if (event.button || event.ctrlKey)
        event.stopImmediatePropagation();
      }
    ).call(d3.zoom().scaleExtent([0.5, 10]).on("zoom", function(event) {
      g.attr("transform", "translate(" + event.transform.x + "," + event.transform.y + ") scale(" + event.transform.k + ")");
    }));
  }
  return g;
}
