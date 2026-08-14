package serve

// Shared IDE JS for polling browser confirms while run is in flight.
const confirmPollJS = `
function buddyConfirmDialog(p) {
  return new Promise((resolve) => {
    const existing = document.getElementById('buddy-confirm-modal');
    if (existing) existing.remove();
    const overlay = document.createElement('div');
    overlay.id = 'buddy-confirm-modal';
    overlay.className = 'buddy-confirm-overlay';
    const def = p.default || p.no;
    overlay.innerHTML =
      '<div class="buddy-confirm-dialog" role="dialog" aria-modal="true">' +
      '<div class="buddy-confirm-q"></div>' +
      '<div class="buddy-confirm-hint"></div>' +
      '<div class="buddy-confirm-actions">' +
      '<button type="button" class="buddy-confirm-no"></button>' +
      '<button type="button" class="buddy-confirm-yes"></button>' +
      '</div></div>';
    overlay.querySelector('.buddy-confirm-q').textContent = p.question || 'Proceed?';
    overlay.querySelector('.buddy-confirm-hint').textContent = p.hint || '';
    const yesBtn = overlay.querySelector('.buddy-confirm-yes');
    const noBtn = overlay.querySelector('.buddy-confirm-no');
    yesBtn.textContent = p.yes || 'y';
    noBtn.textContent = p.no || 'n';
    if (String(def).toLowerCase() === String(p.yes || 'y').toLowerCase()) {
      yesBtn.classList.add('primary');
    } else {
      noBtn.classList.add('primary');
    }
    const done = (answer) => {
      overlay.remove();
      document.removeEventListener('keydown', onKey);
      resolve(answer);
    };
    const onKey = (ev) => {
      if (ev.key === 'Enter') {
        ev.preventDefault();
        done(def);
      } else if (ev.key === 'Escape') {
        ev.preventDefault();
        done(p.no || 'n');
      }
    };
    yesBtn.onclick = () => done(p.yes || 'y');
    noBtn.onclick = () => done(p.no || 'n');
    document.addEventListener('keydown', onKey);
    document.body.appendChild(overlay);
    (String(def).toLowerCase() === String(p.yes || 'y').toLowerCase() ? yesBtn : noBtn).focus();
  });
}

async function withBuddyConfirmPoll(sessionId, workPromise) {
  let stop = false;
  const poll = (async () => {
    while (!stop) {
      try {
        const r = await fetch('/api/session/' + sessionId + '/confirm');
        if (r.status === 204) {
          await new Promise(res => setTimeout(res, 150));
          continue;
        }
        if (!r.ok) {
          await new Promise(res => setTimeout(res, 250));
          continue;
        }
        const p = await r.json();
        const answer = await buddyConfirmDialog(p);
        await fetch('/api/session/' + sessionId + '/confirm', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({answer})
        });
      } catch (_) {
        await new Promise(res => setTimeout(res, 250));
      }
    }
  })();
  try {
    return await workPromise;
  } finally {
    stop = true;
    await poll.catch(() => {});
  }
}
`

const confirmPollCSS = `
.buddy-confirm-overlay {
  position: fixed; inset: 0; z-index: 9999;
  background: rgba(0,0,0,.45);
  display: flex; align-items: center; justify-content: center;
  padding: 1rem;
}
.buddy-confirm-dialog {
  background: var(--panel, #1a222c);
  color: var(--ink, #e7ecf1);
  border: 1px solid var(--border, #334155);
  border-radius: 10px;
  min-width: min(360px, 92vw);
  max-width: 480px;
  padding: 1.1rem 1.2rem 1rem;
  box-shadow: 0 12px 40px rgba(0,0,0,.35);
}
.buddy-confirm-q {
  font-size: 1.05rem; font-weight: 600; margin-bottom: .35rem;
  white-space: pre-wrap;
}
.buddy-confirm-hint {
  color: var(--muted, #8b9aab); font-family: ui-monospace, monospace;
  font-size: .9rem; margin-bottom: 1rem;
}
.buddy-confirm-actions {
  display: flex; gap: .6rem; justify-content: flex-end;
}
.buddy-confirm-actions button {
  appearance: none; border: 1px solid var(--border, #334155);
  background: var(--surface-2, #243040); color: var(--ink, #e7ecf1);
  border-radius: 6px; padding: .45rem .9rem; cursor: pointer; font: inherit;
}
.buddy-confirm-actions button.primary {
  background: var(--accent, #3d9c8f); color: var(--accent-ink, #042f2e);
  border-color: transparent; font-weight: 600;
}
`
