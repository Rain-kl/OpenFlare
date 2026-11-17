// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

let state = Object.freeze({ status: 'loading', progress: 0, message: '' });

export function updateState(change) {
  state = Object.freeze({ ...state, ...change });
  window.dispatchEvent(new CustomEvent('openflare:pow', { detail: state }));
}

window.OpenFlarePoW = Object.freeze({
  get state() { return state; },
  subscribe(callback) {
    callback(state);
    const listener = (event) => callback(event.detail);
    window.addEventListener('openflare:pow', listener);
    return () => window.removeEventListener('openflare:pow', listener);
  },
  retry() { window.location.reload(); },
});

class OpenFlarePoW extends HTMLElement {
  connectedCallback() {
    if (!this.shadowRoot) {
      this.attachShadow({ mode: 'open' }).innerHTML = `
        <style>
        :host{display:block;max-width:340px;font:14px/1.5 system-ui,sans-serif;color:#313131;--surface:#fafafa;--line:#d8d8d8;--accent:#b16286;--success:#238636;--error:#bc3030}
        @media(prefers-color-scheme:dark){:host{color:#eee;--surface:#242424;--line:#555;--accent:#d386b4;--success:#5fbc6d;--error:#ff8080}}
        .box{display:flex;align-items:center;gap:16px;padding:16px;border:1px solid var(--line);border-radius:4px;background:var(--surface);min-height:76px;text-align:left}
        .icon{width:25px;height:25px;flex:none;box-sizing:border-box;border:3px solid var(--line);border-top-color:var(--accent);border-radius:50%;animation:spin 1s linear infinite}
        .brand{margin-left:auto;font-size:10px;letter-spacing:.04em;color:inherit;opacity:.65}
        .track{height:12px;background:var(--line);border-radius:8px;overflow:hidden;margin-top:10px}
        .bar{height:100%;background:var(--accent);width:0;transition:width .25s ease}
        .message{overflow-wrap:anywhere}.value{font-variant-numeric:tabular-nums;white-space:nowrap}
        :host([appearance="turnstile"]) .track,:host([appearance="turnstile"]) .value{display:none}
        :host(:not([appearance="turnstile"])) .box{display:block;border:0;background:transparent;padding:0;text-align:inherit}
        :host(:not([appearance="turnstile"])) .icon,:host(:not([appearance="turnstile"])) .brand{display:none}
        .box[data-status="success"] .icon{border:0;border-radius:0;animation:none;color:var(--success)}
        .box[data-status="success"] .icon::after{content:'✓';font-size:28px;line-height:1}
        .box[data-status="error"] .icon{border:0;animation:none;color:var(--error)}
        .box[data-status="error"] .icon::after{content:'!';font-size:28px;line-height:1}
        button{font:inherit;color:inherit;background:transparent;border:1px solid var(--line);border-radius:4px;padding:5px 12px;margin-top:10px;cursor:pointer}
        [hidden]{display:none!important}@keyframes spin{to{transform:rotate(360deg)}}
        @media(prefers-reduced-motion:reduce){.icon{animation-duration:3s}.bar{transition:none}}
        </style>
        <div class="box" data-status="loading">
          <span class="icon" aria-hidden="true"></span>
          <div style="flex:1;min-width:0">
            <span class="message" role="status" aria-live="polite"></span>
            <span class="value" aria-hidden="true"></span>
            <div class="track" role="progressbar" aria-label="PoW" aria-valuemin="0" aria-valuemax="100"><div class="bar"></div></div>
            <button type="button" hidden></button>
          </div>
          <span class="brand">OpenFlare</span>
        </div>`;
      const retry = this.shadowRoot.querySelector('button');
      retry.addEventListener('click', () => window.OpenFlarePoW.retry());
    }
    this.unsubscribe?.();
    this.unsubscribe = window.OpenFlarePoW.subscribe((value) => {
      const root = this.shadowRoot;
      const progress = Math.round(Math.max(0, Math.min(100, value.progress)));
      root.querySelector('.box').dataset.status = value.status;
      root.querySelector('.message').textContent = value.message;
      root.querySelector('.value').textContent = ` ${progress}%`;
      root.querySelector('.bar').style.width = `${progress}%`;
      root.querySelector('.track').setAttribute('aria-valuenow', String(progress));
      root.querySelector('button').textContent = value.retryLabel || 'Try again';
      root.querySelector('button').hidden = value.status !== 'error';
    });
  }
  disconnectedCallback() { this.unsubscribe?.(); }
}

if (!customElements.get('openflare-pow')) customElements.define('openflare-pow', OpenFlarePoW);
