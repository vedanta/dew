// Lightweight typing-style replay for the demo terminal. No dependencies.
(function () {
  "use strict";

  var body = document.querySelector("#demo .term-body");
  var replayBtn = document.getElementById("replay");
  if (!body) return;

  var lines;
  try {
    lines = JSON.parse(body.getAttribute("data-lines"));
  } catch (e) {
    return;
  }
  body.removeAttribute("data-lines");

  var TYPE_MS = 28; // per character for command lines
  var LINE_PAUSE = 360; // after an output line
  var CMD_PAUSE = 520; // after a typed command, before its output
  var timers = [];

  function clearTimers() {
    timers.forEach(clearTimeout);
    timers = [];
  }
  function at(ms, fn) {
    timers.push(setTimeout(fn, ms));
  }

  function run() {
    clearTimers();
    body.textContent = "";
    var t = 200;

    lines.forEach(function (line) {
      var cls = line.c || "out";

      if (cls === "gap") {
        at(t, function () { body.appendChild(document.createTextNode("\n")); });
        t += 120;
        return;
      }

      if (cls === "cmd") {
        // prompt, then type the command character by character.
        var prompt = line.p || "";
        var span = null;
        at(t, function () {
          if (prompt) {
            var ps = document.createElement("span");
            ps.className = "prompt";
            ps.textContent = prompt;
            body.appendChild(ps);
          }
          span = document.createElement("span");
          span.className = "cmd";
          body.appendChild(span);
        });
        var text = line.t || "";
        for (var i = 0; i < text.length; i++) {
          (function (ch) {
            at(t, function () { span.textContent += ch; });
          })(text[i]);
          t += TYPE_MS;
        }
        at(t, function () { body.appendChild(document.createTextNode("\n")); });
        t += CMD_PAUSE;
        return;
      }

      // output / comment / ok line — appear whole.
      at(t, function () {
        var s = document.createElement("span");
        s.className = cls;
        s.textContent = line.t;
        body.appendChild(s);
        body.appendChild(document.createTextNode("\n"));
      });
      t += LINE_PAUSE;
    });

    // blinking cursor at the end.
    at(t, function () {
      var cur = document.createElement("span");
      cur.className = "cursor";
      cur.textContent = "▍";
      body.appendChild(cur);
      blink(cur);
    });
  }

  function blink(el) {
    var on = true;
    timers.push(setInterval(function () {
      on = !on;
      el.style.visibility = on ? "visible" : "hidden";
    }, 530));
  }

  if (replayBtn) replayBtn.addEventListener("click", run);

  // Copy-to-clipboard for the install banner (and any [data-copy] button).
  document.querySelectorAll("[data-copy]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var text = btn.getAttribute("data-copy");
      var done = function () {
        var orig = btn.textContent;
        btn.textContent = "Copied!";
        btn.classList.add("copied");
        setTimeout(function () { btn.textContent = orig; btn.classList.remove("copied"); }, 1600);
      };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done, done);
      } else {
        done();
      }
    });
  });

  // Keep the version badge in sync with the latest GitHub release. Progressive
  // enhancement: on any failure the static value baked into the HTML stands.
  var pill = document.getElementById("version-pill");
  if (pill && window.fetch) {
    fetch("https://api.github.com/repos/vedanta/dew/releases/latest", {
      headers: { Accept: "application/vnd.github+json" },
    })
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (data) {
        if (data && /^v\d+\.\d+\.\d+/.test(data.tag_name || "")) {
          pill.textContent = data.tag_name;
        }
      })
      .catch(function () {});
  }

  // Run once when the terminal scrolls into view (or immediately as a fallback).
  if ("IntersectionObserver" in window) {
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (en) {
        if (en.isIntersecting) { run(); io.disconnect(); }
      });
    }, { threshold: 0.4 });
    io.observe(document.getElementById("demo"));
  } else {
    run();
  }
})();
