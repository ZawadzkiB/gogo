/* gogo viewer — parse a mermaid-rendered flowchart SVG into a structured model.
 *
 * The hybrid approach (decision D1=A): let the vendored mermaid lay the diagram
 * out once, then read its rendered SVG into
 *   { nodes: [{ id, x, y, w, h, label, classes }],
 *     edges: [{ fromId, toId, label }] }
 * so render.js can own interaction (custom node cards + an owned edge layer that
 * re-routes on drag) instead of shoving mermaid's flat image around.
 *
 * Only flowchart-family diagrams (mermaid `flowchart` / `graph`; gogo `flow` +
 * `use-case`) are parseable here — decision D3. Anything else returns null and
 * the caller falls back to the 0.5.0 pan/zoom canvas (no regression).
 *
 * Reads mermaid v10 conventions (the vendored runtime):
 *   - node group  <g class="node default <classDef...>" data-id="<id>"
 *                    transform="translate(cx,cy)">  (label text inside)
 *   - edge path   <path class="... flowchart-link LS-<from> LE-<to>">
 *
 * Attaches to window.gogoViewer (plain <script>, no import/export — file://).
 */
(function () {
  "use strict";
  var ns = (window.gogoViewer = window.gogoViewer || {});

  // Is this a flowchart-family diagram? Prefer the source declaration; fall back
  // to the rendered SVG's shape so we still detect it if the source is absent.
  function isFlowchart(source, svg) {
    if (source) {
      // Drop mermaid directives (%%{...}%%) and blank lines, then read the kind.
      var lines = source.replace(/%%\{[\s\S]*?\}%%/g, "").split("\n");
      for (var i = 0; i < lines.length; i++) {
        var line = lines[i].trim();
        if (!line || line.indexOf("%%") === 0) continue;
        return /^(flowchart|graph)\b/.test(line);
      }
    }
    if (svg) {
      var rd = svg.getAttribute("aria-roledescription") || "";
      if (/flowchart/.test(rd)) return true;
      if (svg.querySelector("g.nodes g.node")) return true;
    }
    return false;
  }

  // "translate(cx, cy)" or "matrix(a,b,c,d,e,f)" -> {x,y}. Node groups in a
  // flowchart are positioned by their parent-space translate; parsing it (plus
  // getBBox for size) keeps every node in one consistent coordinate space, which
  // is all the owned edge router needs.
  function parseTranslate(transform) {
    if (!transform) return { x: 0, y: 0 };
    var m = /translate\(\s*([-\d.eE]+)[ ,]+([-\d.eE]+)/.exec(transform);
    if (m) return { x: parseFloat(m[1]), y: parseFloat(m[2]) };
    var mm = /matrix\(\s*[-\d.eE]+[ ,]+[-\d.eE]+[ ,]+[-\d.eE]+[ ,]+[-\d.eE]+[ ,]+([-\d.eE]+)[ ,]+([-\d.eE]+)/.exec(transform);
    if (mm) return { x: parseFloat(mm[1]), y: parseFloat(mm[2]) };
    return { x: 0, y: 0 };
  }

  // Custom classDef classes on a node (e.g. proc / art / gate / out / io) — used
  // by render.js to color node cards. Drops mermaid's structural classes.
  var STRUCTURAL = { node: 1, "default": 1, "flowchart-label": 1, clickable: 1, statediagram: 1 };
  function customClasses(el) {
    var out = [];
    var list = el.getAttribute("class") ? el.getAttribute("class").split(/\s+/) : [];
    for (var i = 0; i < list.length; i++) {
      var c = list[i];
      if (c && !STRUCTURAL[c]) out.push(c);
    }
    return out;
  }

  function nodeLabel(g) {
    var el = g.querySelector(".nodeLabel, foreignObject, .label, text") || g;
    // Mermaid renders a multi-line label as separate <tspan>s (or <br>-split
    // spans) with NO whitespace between them, so a naive textContent glues the
    // lines ("with"+"interactive" -> "withinteractive"). Collect each descendant
    // text run and join with a space so line/word boundaries survive.
    var parts = [];
    try {
      var w = document.createTreeWalker(el, NodeFilter.SHOW_TEXT, null);
      var n;
      while ((n = w.nextNode())) {
        var v = (n.nodeValue || "").trim();
        if (v) parts.push(v);
      }
    } catch (e) { /* fall through to textContent */ }
    var txt = parts.length ? parts.join(" ") : (el.textContent || "");
    return txt.replace(/\s+/g, " ").trim();
  }

  // Read a node's box in the parent (nodes-layer) coordinate space: translate +
  // local getBBox. Every node uses the same space, so edges routed from these
  // boxes are internally consistent regardless of any outer group offset.
  function nodeBox(g) {
    var t = parseTranslate(g.getAttribute("transform"));
    var bb;
    try {
      bb = g.getBBox();
    } catch (e) {
      bb = null;
    }
    if (!bb || (!bb.width && !bb.height)) return null;
    return { x: t.x + bb.x, y: t.y + bb.y, w: bb.width, h: bb.height };
  }

  // Strip the LS-/LE- prefix carried on a flowchart edge path's class list.
  function edgeEndpoints(path) {
    var from = null, to = null;
    var list = path.getAttribute("class") ? path.getAttribute("class").split(/\s+/) : [];
    for (var i = 0; i < list.length; i++) {
      var c = list[i];
      if (c.indexOf("LS-") === 0) from = c.slice(3);
      else if (c.indexOf("LE-") === 0) to = c.slice(3);
    }
    // Fallback: edge id is "L-<from>-<to>-<n>"; only usable when node ids carry
    // no hyphens (true for gogo diagrams). Endpoints are validated against the
    // known node-id set by the caller, so a bad split is dropped, never guessed.
    if ((!from || !to) && path.id && path.id.indexOf("L-") === 0) {
      var parts = path.id.slice(2).split("-");
      if (parts.length >= 3) {
        from = from || parts[0];
        to = to || parts[1];
      }
    }
    return { from: from, to: to };
  }

  function parseMermaid(svg, source) {
    if (!svg || !isFlowchart(source, svg)) return null;

    var groups = svg.querySelectorAll("g.node");
    if (!groups.length) return null;

    var nodes = [];
    var known = {};
    for (var i = 0; i < groups.length; i++) {
      var g = groups[i];
      var id = g.getAttribute("data-id") || g.getAttribute("id");
      if (!id) continue;
      var box = nodeBox(g);
      if (!box) continue;
      nodes.push({
        id: id,
        x: box.x,
        y: box.y,
        w: box.w,
        h: box.h,
        label: nodeLabel(g),
        classes: customClasses(g)
      });
      known[id] = true;
    }
    if (!nodes.length) return null;

    // Edge labels appear in document order alongside edge paths (one slot per
    // source edge). Key each label to the edge's own ordinal among the source
    // paths — advanced for every unique path examined, even one we drop — so a
    // dropped edge consumes its own label slot instead of shifting every
    // subsequent label by one (best-effort — labels are optional to the model).
    var labelEls = svg.querySelectorAll(".edgeLabels .edgeLabel");
    var edges = [];
    var paths = svg.querySelectorAll(".edgePaths path.flowchart-link, .edgePaths > path, path.flowchart-link");
    var seen = {};
    var edgeOrd = 0;
    for (var j = 0; j < paths.length; j++) {
      var p = paths[j];
      if (p.id && seen[p.id]) continue; // de-dupe if a path matched two selectors
      if (p.id) seen[p.id] = true;
      var slot = edgeOrd++;             // this path's position among source edges
      var ep = edgeEndpoints(p);
      if (!ep.from || !ep.to || !known[ep.from] || !known[ep.to]) continue; // dropped — slot already spent
      var lblEl = labelEls[slot];
      var lbl = lblEl ? (lblEl.textContent || "").replace(/\s+/g, " ").trim() : "";
      edges.push({ fromId: ep.from, toId: ep.to, label: lbl });
    }

    return { nodes: nodes, edges: edges };
  }

  ns.isFlowchart = isFlowchart;
  ns.parseMermaid = parseMermaid;
})();
