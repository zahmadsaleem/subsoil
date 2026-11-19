(() => {
  'use strict';
  const element = id => document.getElementById(id);
  const slides = Array.from(document.querySelectorAll('.slide'));
  const viewport = element('viewport');
  const deck = element('deck');
  const notesButton = element('notes-toggle');
  const notesPanel = element('notes-panel');
  const help = element('help');
  const previous = element('prev');
  const next = element('next');
  let current = 0;
  let touchStart = null;

  function fit() {
    const scale = Math.min(viewport.clientWidth / 1600, viewport.clientHeight / 900);
    deck.style.transform = `scale(${scale})`;
  }

  function hashIndex() {
    const match = location.hash.match(/^#(?:slide-)?(\d+)$/);
    return match ? Number(match[1]) - 1 : 0;
  }

  function show(index, updateHash = true) {
    current = Math.max(0, Math.min(slides.length - 1, index));
    const slide = slides[current];
    for (const candidate of slides) {
      const active = candidate === slide;
      candidate.classList.toggle('active', active);
      candidate.setAttribute('aria-hidden', String(!active));
      candidate.inert = !active;
    }
    const number = current + 1;
    const title = slide.dataset.title;
    element('position').textContent = `${String(number).padStart(2, '0')} / ${slides.length}`;
    element('live').textContent = `Slide ${number} of ${slides.length}. ${title}`;
    element('notes-title').textContent = title;
    element('notes-content').innerHTML = slide.querySelector('.speaker-note').innerHTML;
    document.title = `${number}/${slides.length} · ${title} · Open CDE governance`;
    document.querySelector('.progress').style.width = `${number / slides.length * 100}%`;
    previous.disabled = current === 0;
    next.disabled = current === slides.length - 1;
    if (updateHash) history.replaceState(null, '', `#slide-${number}`);
  }

  function toggleNotes() {
    const open = document.body.classList.toggle('notes-open');
    notesButton.setAttribute('aria-pressed', String(open));
    notesPanel.setAttribute('aria-hidden', String(!open));
    fit();
  }

  async function toggleFullscreen() {
    try {
      if (document.fullscreenElement) await document.exitFullscreen();
      else await document.documentElement.requestFullscreen();
    } catch {
      element('live').textContent = 'Fullscreen is unavailable in this browser.';
    }
  }

  previous.addEventListener('click', () => show(current - 1));
  next.addEventListener('click', () => show(current + 1));
  notesButton.addEventListener('click', toggleNotes);
  element('fullscreen').addEventListener('click', toggleFullscreen);
  element('print').addEventListener('click', () => window.print());
  element('help-open').addEventListener('click', () => help.showModal());
  element('help-close').addEventListener('click', () => help.close());
  document.addEventListener('keydown', event => {
    if (help.open || event.altKey || event.ctrlKey || event.metaKey ||
        /INPUT|TEXTAREA|SELECT/.test(event.target.tagName)) return;
    switch (event.key) {
      case ' ':
        if (event.target.closest('button, a')) return;
        event.preventDefault();
        show(current + (event.shiftKey ? -1 : 1));
        break;
      case 'ArrowRight': case 'ArrowDown': case 'PageDown':
        event.preventDefault();
        show(current + 1);
        break;
      case 'ArrowLeft': case 'ArrowUp': case 'PageUp':
        event.preventDefault();
        show(current - 1);
        break;
      case 'Home': event.preventDefault(); show(0); break;
      case 'End': event.preventDefault(); show(slides.length - 1); break;
      case 'n': case 'N': toggleNotes(); break;
      case 'f': case 'F': toggleFullscreen(); break;
      case '?': help.showModal(); break;
    }
  });
  viewport.addEventListener('touchstart', event => {
    if (event.touches.length === 1) touchStart = event.touches[0];
  }, {passive: true});
  viewport.addEventListener('touchend', event => {
    if (!touchStart) return;
    const touch = event.changedTouches[0];
    const dx = touch.clientX - touchStart.clientX;
    const dy = touch.clientY - touchStart.clientY;
    if (Math.abs(dx) > 55 && Math.abs(dx) > Math.abs(dy) * 1.5) show(current + (dx < 0 ? 1 : -1));
    touchStart = null;
  }, {passive: true});
  window.addEventListener('beforeprint', () => {
    for (const slide of slides) {
      slide.inert = false;
      slide.removeAttribute('aria-hidden');
    }
  });
  window.addEventListener('afterprint', () => show(current, false));
  window.addEventListener('resize', fit);
  window.addEventListener('hashchange', () => show(hashIndex(), false));
  show(hashIndex(), false);
  fit();
})();
