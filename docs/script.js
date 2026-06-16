/* ============================================================
   Chrome History Cleaner — landing page behaviour
   Vanilla JS, no dependencies. Every feature is feature-detected
   and degrades gracefully if an element is missing.
   ============================================================ */
(function () {
  'use strict';

  /* ---------- Navbar: add .scrolled past a threshold ---------- */
  var navbar = document.getElementById('navbar');
  if (navbar) {
    var onScroll = function () {
      navbar.classList.toggle('scrolled', window.scrollY > 20);
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
  }

  /* ---------- Mobile navigation toggle ---------- */
  var navToggle = document.getElementById('navToggle');
  var navLinks = document.getElementById('navLinks');
  if (navToggle && navLinks) {
    var closeMenu = function () {
      navToggle.classList.remove('open');
      navLinks.classList.remove('open');
    };
    navToggle.addEventListener('click', function () {
      navToggle.classList.toggle('open');
      navLinks.classList.toggle('open');
    });
    navLinks.querySelectorAll('a').forEach(function (a) {
      a.addEventListener('click', closeMenu);
    });
  }

  /* ---------- Install tabs ---------- */
  var tabs = document.querySelectorAll('.install-tab');
  var panels = document.querySelectorAll('.install-panel');
  tabs.forEach(function (tab) {
    tab.addEventListener('click', function () {
      var target = tab.getAttribute('data-tab');
      tabs.forEach(function (t) { t.classList.remove('active'); });
      panels.forEach(function (p) { p.classList.remove('active'); });
      tab.classList.add('active');
      var panel = document.getElementById('panel-' + target);
      if (panel) panel.classList.add('active');
    });
  });

  /* ---------- Copy-to-clipboard buttons ---------- */
  var copyText = function (text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      return navigator.clipboard.writeText(text);
    }
    // Legacy fallback for non-secure contexts.
    return new Promise(function (resolve, reject) {
      try {
        var ta = document.createElement('textarea');
        ta.value = text;
        ta.style.position = 'fixed';
        ta.style.opacity = '0';
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
        resolve();
      } catch (e) { reject(e); }
    });
  };

  document.querySelectorAll('.copy-btn').forEach(function (btn) {
    var original = btn.innerHTML;
    btn.addEventListener('click', function () {
      var text = btn.getAttribute('data-copy') || '';
      copyText(text).then(function () {
        btn.classList.add('copied');
        btn.textContent = 'Copied!';
        setTimeout(function () {
          btn.classList.remove('copied');
          btn.innerHTML = original;
        }, 1600);
      }).catch(function () {
        btn.textContent = 'Press Ctrl+C';
        setTimeout(function () { btn.innerHTML = original; }, 1600);
      });
    });
  });

  /* ---------- Scroll-reveal for .fade-in elements ---------- */
  var faders = document.querySelectorAll('.fade-in');
  if ('IntersectionObserver' in window && faders.length) {
    var io = new IntersectionObserver(function (entries, obs) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          entry.target.classList.add('visible');
          obs.unobserve(entry.target);
        }
      });
    }, { threshold: 0.12, rootMargin: '0px 0px -40px 0px' });
    faders.forEach(function (el) { io.observe(el); });
  } else {
    // No IntersectionObserver — just reveal everything.
    faders.forEach(function (el) { el.classList.add('visible'); });
  }

  /* ---------- Hero terminal typewriter ---------- */
  var body = document.getElementById('terminalBody');
  var cursor = document.getElementById('terminalCursor');
  var reduceMotion = window.matchMedia &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  if (body) {
    var script = [
      { t: 'cmd', text: 'chrome-cleaner -site tracker.com -dry-run' },
      { t: 'out', text: "--- Impact Report for 'tracker.com' (Profile: Default) ---" },
      { t: 'out', text: 'History URLs:          42' },
      { t: 'out', text: 'Visit Records:         128' },
      { t: 'out', text: 'Segments:              9' },
      { t: 'out', text: 'Keyword Search Terms:  4' },
      { t: 'out', text: 'Search Keywords:       2' },
      { t: 'out', text: 'Autofill entries:      6' },
      { t: 'ok',  text: 'Dry run completed. No data was modified.' }
    ];

    var addLine = function (cls, text) {
      var line = document.createElement('div');
      line.className = 'terminal-line';
      if (cls) {
        var span = document.createElement('span');
        span.className = cls;
        span.textContent = text;
        line.appendChild(span);
      } else {
        line.textContent = text;
      }
      body.insertBefore(line, cursor ? cursor.parentNode : null);
      return line;
    };

    var renderAll = function () {
      script.forEach(function (step) {
        if (step.t === 'cmd') {
          var l = addLine(null, '');
          l.innerHTML = '<span class="terminal-prompt">$</span> ' +
            step.text.replace(/[&<>]/g, function (c) {
              return { '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c];
            });
        } else {
          addLine(step.t === 'ok' ? 'terminal-success' : 'terminal-output', step.text);
        }
      });
    };

    if (reduceMotion) {
      renderAll();
    } else {
      var i = 0;
      var runStep = function () {
        if (i >= script.length) return;
        var step = script[i++];
        if (step.t === 'cmd') {
          var l = addLine(null, '');
          var prompt = '<span class="terminal-prompt">$</span> ';
          var typed = 0;
          var typeChar = function () {
            typed++;
            var shown = step.text.slice(0, typed).replace(/[&<>]/g, function (c) {
              return { '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c];
            });
            l.innerHTML = prompt + shown;
            if (typed < step.text.length) {
              setTimeout(typeChar, 38);
            } else {
              setTimeout(runStep, 360);
            }
          };
          typeChar();
        } else {
          addLine(step.t === 'ok' ? 'terminal-success' : 'terminal-output', step.text);
          setTimeout(runStep, 130);
        }
      };
      // Kick off shortly after load so the hero settles first.
      setTimeout(runStep, 700);
    }
  }
})();
