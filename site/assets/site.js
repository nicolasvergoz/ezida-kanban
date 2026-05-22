/* =========================================================
   Ezida — landing page scripts
   Vanilla JS. Theme + accent + terminal animation + copy.
   ========================================================= */
(function () {
  'use strict';

  /* ---------- Copy button ---------- */
  function initCopy() {
    document.querySelectorAll('[data-copy-target]').forEach((btn) => {
      btn.addEventListener('click', () => {
        const target = document.querySelector(btn.dataset.copyTarget);
        if (!target) return;
        const text = target.textContent.trim();
        navigator.clipboard.writeText(text).then(() => {
          const original = btn.textContent;
          btn.textContent = 'Copied';
          btn.classList.add('copied');
          setTimeout(() => {
            btn.textContent = original;
            btn.classList.remove('copied');
          }, 1600);
        }).catch(() => {
          // fallback: select
          const range = document.createRange();
          range.selectNode(target);
          window.getSelection().removeAllRanges();
          window.getSelection().addRange(range);
        });
      });
    });
  }

  /* ---------- Terminal animation ---------- */
  // Steps grammar:
  //   { p: true }                  → write a fresh "$ " prompt
  //   { type: 'cmd text', delay?, speed? }  → type characters into current line
  //   { out: 'text', cls?, delay? }→ print an output line
  //   { pause: ms }                → wait
  //   { clear: true }              → clear the terminal body
  const SCRIPT = [
    { p: true },
    { type: 'ezida init', speed: 55 },
    { pause: 400 },
    { out: 'created kanban.toml', cls: 'ok', icon: '✓' },
    { out: 'created .claude/skills/ezida-kanban/SKILL.md', cls: 'ok', icon: '✓' },
    { out: '', delay: 0 },
    { out: '# board ready. commit both files.', cls: 'comment' },
    { pause: 1200 },
    { p: true },
    { type: 'ezida add "Refactor auth" --column=todo --priority=high --tags=security', speed: 28 },
    { pause: 350 },
    { out: 'a3f2k9 → todo', cls: 'ok', icon: '✓', tags: ['high', 'security'] },
    { pause: 1000 },
    { p: true },
    { type: 'ezida move a3f2k9 ongoing', speed: 38 },
    { pause: 250 },
    { out: 'a3f2k9 → ongoing', cls: 'ok', icon: '✓' },
    { pause: 900 },
    { p: true },
    { type: 'ezida list --column=ongoing', speed: 38 },
    { pause: 200 },
    { out: 'ONGOING (1)', cls: 'key' },
    { out: '  a3f2k9  Refactor auth   [high] [security]', cls: 'out' },
    { pause: 2000 },
    { p: true },
    { type: 'ezida serve', speed: 55 },
    { pause: 400 },
    { out: 'serving kanban.toml on http://127.0.0.1:7777', cls: 'key' },
    { out: 'watching kanban.toml for changes…', cls: 'comment' },
    { pause: 3500 },
    { clear: true }
  ];

  function el(tag, cls, text) {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }

  function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

  async function runTerminal(host) {
    const body = host.querySelector('.terminal-body');
    if (!body) return;
    let stopped = false;
    let paused = host.dataset.paused === 'true';

    // observer to pause when off-screen
    let inView = true;
    const io = new IntersectionObserver((entries) => {
      entries.forEach((e) => { inView = e.isIntersecting; });
    }, { threshold: 0.05 });
    io.observe(host);

    // listen for pause toggle
    host.addEventListener('terminal:pause', (e) => { paused = !!e.detail; });
    host.addEventListener('terminal:stop', () => { stopped = true; });

    while (!stopped) {
      body.innerHTML = '';
      let line = null;
      let cursor = null;

      function ensureLine() {
        if (cursor && cursor.parentNode) cursor.remove();
        line = el('span', 'term-line');
        cursor = el('span', 'term-cursor');
        body.appendChild(line);
        body.appendChild(cursor);
      }

      ensureLine();
      const prompt = el('span', 'term-prompt', '$');
      line.appendChild(prompt);
      line.appendChild(document.createTextNode(' '));

      for (const step of SCRIPT) {
        while (paused || !inView) {
          await sleep(150);
          if (stopped) return;
        }
        if (stopped) return;

        if (step.clear) {
          await sleep(step.delay || 200);
          break; // restart loop
        }
        if (step.p) {
          // finish current line, start new prompt line
          if (cursor && cursor.parentNode) cursor.remove();
          ensureLine();
          line.appendChild(el('span', 'term-prompt', '$'));
          line.appendChild(document.createTextNode(' '));
          await sleep(step.delay || 300);
          continue;
        }
        if (step.type) {
          // type characters into current line
          const cmdSpan = el('span', 'term-cmd');
          line.appendChild(cmdSpan);
          const speed = step.speed || 40;
          for (const ch of step.type) {
            cmdSpan.appendChild(document.createTextNode(ch));
            await sleep(speed + (Math.random() * 25 - 12));
            if (stopped) return;
          }
          // newline after typing
          if (cursor && cursor.parentNode) cursor.remove();
          continue;
        }
        if (step.out != null) {
          if (step.delay) await sleep(step.delay);
          const outLine = el('div', 'term-line');
          const cls = 'term-out' + (step.cls ? ' term-out-' + step.cls : '');
          const span = el('span', cls);
          if (step.icon) {
            const ic = el('span', '', step.icon + ' ');
            ic.style.color = 'oklch(0.7 0.13 145)';
            outLine.appendChild(ic);
          }
          if (step.tags && step.tags.length) {
            // Build a richer line: leading text, then tags
            span.textContent = step.out + '  ';
            outLine.appendChild(span);
            step.tags.forEach((t) => {
              const tag = el('span', 'term-tag' + (t === 'high' ? ' high' : ''), t);
              outLine.appendChild(tag);
            });
          } else {
            span.textContent = step.out;
            outLine.appendChild(span);
          }
          body.appendChild(outLine);
          // ensure cursor at the end on a fresh empty line
          if (cursor && cursor.parentNode) cursor.remove();
          line = el('span', 'term-line');
          cursor = el('span', 'term-cursor');
          body.appendChild(line);
          body.appendChild(cursor);
          continue;
        }
        if (step.pause) {
          await sleep(step.pause);
          continue;
        }
      }
      await sleep(800);
    }
  }

  function initTerminals() {
    document.querySelectorAll('[data-terminal]').forEach((host) => {
      // Skip if disabled in body
      if (document.body.dataset.terminal === 'off') return;
      runTerminal(host);
    });
  }

  /* ---------- Active section in nav ---------- */
  function initScrollSpy() {
    const links = document.querySelectorAll('.nav-links a[href^="#"]');
    if (!links.length) return;
    const sections = Array.from(links).map((l) => document.querySelector(l.getAttribute('href'))).filter(Boolean);
    const obs = new IntersectionObserver((entries) => {
      entries.forEach((e) => {
        if (e.isIntersecting) {
          links.forEach((l) => l.classList.toggle('active', l.getAttribute('href') === '#' + e.target.id));
        }
      });
    }, { rootMargin: '-40% 0px -55% 0px' });
    sections.forEach((s) => obs.observe(s));
  }

  /* ---------- Boot ---------- */
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', boot);
  } else {
    boot();
  }
  function boot() {
    initCopy();
    initTerminals();
    initScrollSpy();
  }
})();
