import { LitElement, html, css } from 'https://cdn.jsdelivr.net/gh/lit/dist@3/core/lit-core.min.js';
import { sharedStyles, cssVariables } from '../styles/shared-styles.js';

export class ModalDialog extends LitElement {
  static properties = {
    open: { type: Boolean },
    title: { type: String },
    content: { type: String }
  };

  static styles = [
    sharedStyles,
    cssVariables,
    css`
    :host {
      display: none;
      position: fixed;
      top: 0;
      left: 0;
      width: 100%;
      height: 100%;
      background: rgba(0, 0, 0, 0.7);
      z-index: 1000;
      align-items: center;
      justify-content: center;
    }

    :host([open]) {
      display: flex;
    }

    .modal-content {
      background: var(--bg-surface);
      border-radius: var(--radius-lg);
      box-shadow: var(--shadow-xl);
      min-width: 324px;
      max-width: 90vw;
      max-height: 80vh;
      width: fit-content;
      display: flex;
      flex-direction: column;
      border: 1px solid var(--border-color);
    }

    @media (max-width: 768px) {
      .modal-content {
        min-width: 300px;
        max-width: 95vw;
      }
    }

    .modal-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 20px 24px;
      border-bottom: 1px solid var(--border-color);
    }

    .modal-header h3 {
      margin: 0;
      font-size: 18px;
      font-weight: 600;
      color: var(--text-primary);
    }

    .modal-close {
      background: transparent;
      border: none;
      color: var(--text-secondary);
      cursor: pointer;
      padding: 4px;
      display: flex;
      align-items: center;
      justify-content: center;
      border-radius: var(--radius-sm);
      transition: var(--transition);
    }

    .modal-close:hover {
      background: var(--bg-hover);
      color: var(--text-primary);
    }

    .modal-body {
      padding: 24px;
      overflow-y: auto;
      flex: 1;
    }

    pre {
      /*background: var(--bg-elevated);*/
      padding: 16px;
      border-radius: var(--radius-md);
      overflow-x: auto;
      color: var(--text-primary);
      font-family: 'Courier New', monospace;
      font-size: 13px;
      line-height: 1.5;
      margin: 0;
      white-space: pre-wrap;
      word-wrap: break-word;
    }
  `
  ];

  constructor() {
    super();
    this.open = false;
    this.title = 'Content';
    this.content = '';
    this._handleKeyDown = this._handleKeyDown.bind(this);
  }

  connectedCallback() {
    super.connectedCallback();
    document.addEventListener('keydown', this._handleKeyDown);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    document.removeEventListener('keydown', this._handleKeyDown);
  }

  updated(changedProperties) {
    if (changedProperties.has('open')) {
      if (this.open) {
        this.setAttribute('open', '');
      } else {
        this.removeAttribute('open');
      }
    }
  }

  _handleKeyDown(e) {
    if (this.open && e.key === 'Escape') {
      this.close();
    }
  }

  close() {
    this.open = false;
    this.dispatchEvent(new CustomEvent('modal-close', {
      bubbles: true,
      composed: true
    }));
  }

  render() {
    return html`
      <div class="modal-content" @click=${(e) => e.stopPropagation()}>
        <div class="modal-header">
          <h3>${this.title}</h3>
          <button class="modal-close" @click=${this.close}>
            <svg
              xmlns="http://www.w3.org/2000/svg"
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>
        <div class="modal-body">
          <pre>${this.content}</pre>
        </div>
      </div>
    `;
  }
}

customElements.define('srdb-modal-dialog', ModalDialog);
