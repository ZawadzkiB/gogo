/* gogo viewer — pure canvas geometry (no DOM, no deps).
 *
 * Ported from xplan's web/src/components/diagramGeometry.ts: border-anchoring
 * and an orthogonal ("elbow") edge router so connector lines leave/enter node
 * borders perpendicularly and never cross a box interior. Re-runs every frame
 * from live node boxes, so edges re-route as nodes are dragged.
 *
 * Attaches to the single shared namespace window.gogoViewer (loaded as a plain
 * <script> tag — no ES module import/export, because file:// blocks both).
 */
(function () {
  "use strict";
  var ns = (window.gogoViewer = window.gogoViewer || {});

  function clamp(v, lo, hi) {
    return Math.max(lo, Math.min(hi, v));
  }

  // Pick the border point of `box` on the side facing `toward`, and report which
  // side it is so the router can choose an orthogonal path that leaves/enters
  // perpendicular to the box edge (arrowhead sits flush on the border).
  function borderAnchor(box, toward) {
    var cx = box.x + box.w / 2;
    var cy = box.y + box.h / 2;
    var dx = toward.x - cx;
    var dy = toward.y - cy;
    // Compare against the box aspect so wide boxes still exit left/right sensibly.
    if (Math.abs(dx) * box.h >= Math.abs(dy) * box.w) {
      return { pt: { x: cx + (dx >= 0 ? box.w / 2 : -box.w / 2), y: cy }, side: dx >= 0 ? "r" : "l" };
    }
    return { pt: { x: cx, y: cy + (dy >= 0 ? box.h / 2 : -box.h / 2) }, side: dy >= 0 ? "b" : "t" };
  }

  function horizontal(s) {
    return s === "l" || s === "r";
  }

  // Orthogonal ("elbow") path between two boxes, anchored on their borders so the
  // line never crosses either node interior. Returns an SVG path string, the mid
  // point (for edge labels), and both endpoints.
  function routeEdge(a, b) {
    var ca = { x: a.x + a.w / 2, y: a.y + a.h / 2 };
    var cb = { x: b.x + b.w / 2, y: b.y + b.h / 2 };
    var from = borderAnchor(a, cb);
    var to = borderAnchor(b, ca);
    var p = from.pt;
    var q = to.pt;

    var d, mid;
    if (horizontal(from.side) && horizontal(to.side)) {
      var mx = (p.x + q.x) / 2;
      d = "M " + p.x + " " + p.y + " L " + mx + " " + p.y + " L " + mx + " " + q.y + " L " + q.x + " " + q.y;
      mid = { x: mx, y: (p.y + q.y) / 2 };
    } else if (!horizontal(from.side) && !horizontal(to.side)) {
      var my = (p.y + q.y) / 2;
      d = "M " + p.x + " " + p.y + " L " + p.x + " " + my + " L " + q.x + " " + my + " L " + q.x + " " + q.y;
      mid = { x: (p.x + q.x) / 2, y: my };
    } else if (horizontal(from.side)) {
      // a exits sideways, b enters vertically -> single elbow at (q.x, p.y)
      d = "M " + p.x + " " + p.y + " L " + q.x + " " + p.y + " L " + q.x + " " + q.y;
      mid = { x: q.x, y: p.y };
    } else {
      // a exits vertically, b enters sideways -> single elbow at (p.x, q.y)
      d = "M " + p.x + " " + p.y + " L " + p.x + " " + q.y + " L " + q.x + " " + q.y;
      mid = { x: p.x, y: q.y };
    }
    return { d: d, mid: mid, from: p, to: q };
  }

  // Bounding box of a list of already-positioned boxes ({x,y,w,h}), padded so
  // content doesn't sit flush against the canvas edge. Falls back to a sane box
  // when there are no boxes (mirrors xplan's contentBounds).
  function contentBounds(boxes, pad) {
    if (pad == null) pad = 60;
    if (!boxes || !boxes.length) return { x: 0, y: 0, w: 600, h: 400 };
    var minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (var i = 0; i < boxes.length; i++) {
      var b = boxes[i];
      if (b.x < minX) minX = b.x;
      if (b.y < minY) minY = b.y;
      if (b.x + b.w > maxX) maxX = b.x + b.w;
      if (b.y + b.h > maxY) maxY = b.y + b.h;
    }
    return { x: minX - pad, y: minY - pad, w: maxX - minX + pad * 2, h: maxY - minY + pad * 2 };
  }

  ns.geometry = {
    clamp: clamp,
    borderAnchor: borderAnchor,
    routeEdge: routeEdge,
    contentBounds: contentBounds
  };
})();
