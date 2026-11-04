(function () {
  var nav = document.querySelector('.nav');
  if (nav) addEventListener('scroll', function () { nav.classList.toggle('scrolled', scrollY > 8); }, { passive: true });

  var io = 'IntersectionObserver' in window && new IntersectionObserver(function (es) {
    es.forEach(function (e) { if (e.isIntersecting) { e.target.classList.add('in'); io.unobserve(e.target); } });
  }, { threshold: .08 });
  document.querySelectorAll('.rv').forEach(function (el) { io ? io.observe(el) : el.classList.add('in'); });

  function copy(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) return navigator.clipboard.writeText(text).catch(function () {});
    var a = document.createElement('textarea');
    a.value = text;
    document.body.appendChild(a);
    a.select();
    try { document.execCommand('copy'); } catch (e) {}
    a.remove();
    return Promise.resolve();
  }

  document.querySelectorAll('button.copy').forEach(function (b) {
    var label = b.textContent;
    b.addEventListener('click', function () {
      copy(b.getAttribute('data-copy')).then(function () {
        b.textContent = b.getAttribute('data-done');
        setTimeout(function () { b.textContent = label; }, 1600);
      });
    });
  });

  var v = document.getElementById('promo'), vc = document.getElementById('vctl');
  if (v && vc) {
    var sync = function () {
      vc.classList.toggle('paused', v.paused);
      vc.setAttribute('aria-label', vc.getAttribute(v.paused ? 'data-play' : 'data-pause'));
    };
    vc.addEventListener('click', function () { v.paused ? v.play() : v.pause(); });
    v.addEventListener('play', sync);
    v.addEventListener('pause', sync);
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) { v.removeAttribute('autoplay'); v.pause(); }
    sync();
  }

  var stage = document.getElementById('demo');
  if (!stage) return;
  var files = stage.querySelector('.files'), menu = stage.querySelector('.ctx'), toast = stage.querySelector('.toast');
  var d = stage.dataset, timer;
  var touch = matchMedia('(hover: none)').matches;

  function esc(s) { return s.replace(/[&<>"]/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]; }); }
  function say(tpl, key, value) {
    toast.innerHTML = esc(tpl).replace(key, '<code>' + esc(value) + '</code>');
    toast.classList.add('show');
    clearTimeout(timer);
    timer = setTimeout(function () { toast.classList.remove('show'); }, 2400);
  }
  function closeSubs() { menu.querySelectorAll('li.open').forEach(function (li) { li.classList.remove('open'); }); }
  function hide() { menu.hidden = true; closeSubs(); }
  function select(el) {
    files.querySelectorAll('.file.sel').forEach(function (f) { f.classList.remove('sel'); });
    if (el) el.classList.add('sel');
  }
  function show(x, y) {
    closeSubs();
    menu.hidden = false;
    menu.classList.remove('flip');
    var W = files.clientWidth, H = files.clientHeight, mw = menu.offsetWidth, mh = menu.offsetHeight;
    var sub = innerWidth > 640 ? 204 : 0;
    x = Math.max(6, Math.min(x, W - mw - 6));
    y = Math.max(6, Math.min(y, H - mh - 6));
    if (sub && x + mw + sub > W) menu.classList.add('flip');
    menu.style.left = x + 'px';
    menu.style.top = y + 'px';
  }
  function at(e) {
    var r = files.getBoundingClientRect();
    return [e.clientX - r.left, e.clientY - r.top];
  }

  files.addEventListener('contextmenu', function (e) {
    if (menu.contains(e.target)) { e.preventDefault(); return; }
    e.preventDefault();
    select(null);
    var p = at(e);
    show(p[0], p[1]);
  });

  files.addEventListener('click', function (e) {
    if (menu.contains(e.target)) return;
    var f = e.target.closest('.file');
    if (!menu.hidden) { hide(); select(f); return; }
    if (f) { select(f); return; }
    select(null);
    if (touch) { var p = at(e); show(p[0], p[1]); }
  });

  files.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') { hide(); files.focus(); return; }
    if (e.target === files && (e.key === 'ContextMenu' || e.key === 'Enter' || e.key === ' ' || (e.shiftKey && e.key === 'F10'))) {
      e.preventDefault();
      show(files.clientWidth / 2 - 118, 40);
      var first = menu.querySelector('button[data-first]');
      if (first) first.focus();
    }
  });

  menu.addEventListener('click', function (e) {
    var b = e.target.closest('button');
    if (!b) return;
    var li = b.parentElement;
    if (li.querySelector('.sub')) {
      var was = li.classList.contains('open');
      closeSubs();
      if (!was) li.classList.add('open');
      return;
    }
    var act = b.getAttribute('data-act');
    hide();
    if (act === 'new') {
      var ext = b.getAttribute('data-ext'), names = {}, n = 1, name;
      files.querySelectorAll('.file span').forEach(function (s) { names[s.textContent] = 1; });
      do { name = 'Untitled' + (n > 1 ? ' ' + n : '') + ext; n++; } while (names[name]);
      var el = document.createElement('div');
      el.className = 'file new';
      el.innerHTML = '<div class="ico">' + document.getElementById('doc-ico').innerHTML + '</div><span></span>';
      el.querySelector('span').textContent = name;
      files.insertBefore(el, menu);
      select(el);
      say(d.tCreated, '{name}', name);
    } else if (act === 'copy') {
      var text = b.getAttribute('data-text');
      copy(text);
      say(d.tCopied, '{text}', text);
    } else if (act === 'open') {
      say(d.tOpen, '{app}', b.getAttribute('data-app'));
    } else if (act === 'hidden') {
      var on = stage.classList.toggle('show-hidden');
      files.querySelectorAll('.hidden-file').forEach(function (f) { f.hidden = !on; });
      b.querySelector('.t').textContent = on ? d.tHide : d.tShow;
    }
  });

  var start = menu.querySelector('li.open');
  if (innerWidth > 640) show(files.clientWidth * .31, 124);
  else show(files.clientWidth - menu.offsetWidth - 8, 232);
  if (start) start.classList.add('open');

  document.addEventListener('pointerdown', function (e) { if (!stage.contains(e.target)) hide(); });
  stage.addEventListener('mouseover', function (e) {
    var li = [].find.call(menu.children, function (c) { return c.contains(e.target); });
    if (li && !li.classList.contains('open')) closeSubs();
  });
})();
