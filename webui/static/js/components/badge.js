import { LitElement, html, css } from 'lit';
import { sharedStyles, cssVariables } from '~/common/shared-styles.js';

export class Badge extends LitElement {
  static properties = {
    variant: { type: String }, // 'primary', 'success', 'warning', 'danger', 'info'
    icon: { type: String },
    size: { type: String } // 'sm', 'md'
  };

  static styles = [
    sharedStyles,
    cssVariables,
    css`
      :host {
        display: inline-flex;
      }

      .badge {
        position: relative;
        display: inline-flex;
        align-items: center;
        gap: 4px;
        padding: 2px 8px;
        font-size: 11px;
        font-weight: 600;
        border-radius: var(--radius-sm);
        white-space: nowrap;
      }

      .badge.size-md {
        padding: 4px 10px;
        font-size: 12px;
      }

      /* Primary variant */
      .badge.variant-primary {
        --badge-border-color: rgba(99, 102, 241, 0.2);
        background: rgba(99, 102, 241, 0.15);
        color: var(--primary);
      }

      /* Success variant */
      .badge.variant-success {
        --badge-border-color: rgba(16, 185, 129, 0.2);
        background: rgba(16, 185, 129, 0.15);
        color: var(--success);
      }

      /* Warning variant */
      .badge.variant-warning {
        --badge-border-color: rgba(245, 158, 11, 0.2);
        background: rgba(245, 158, 11, 0.15);
        color: var(--warning);
      }

      /* Danger variant */
      .badge.variant-danger {
        --badge-border-color: rgba(239, 68, 68, 0.2);
        background: rgba(239, 68, 68, 0.15);
        color: var(--danger);
      }

      /* Info variant */
      .badge.variant-info {
        --badge-border-color: rgba(59, 130, 246, 0.2);
        background: rgba(59, 130, 246, 0.15);
        color: var(--info);
      }

      /* Secondary variant */
      .badge.variant-secondary {
        --badge-border-color: rgba(160, 160, 176, 0.2);
        background: rgba(160, 160, 176, 0.15);
        color: var(--text-secondary);
      }

      .icon {
        font-size: 12px;
        line-height: 1;
      }

      .badge.size-md .icon {
        font-size: 14px;
      }
    `
  ];

  constructor() {
    super();
    this.variant = 'primary';
    this.icon = '';
    this.size = 'sm';
  }

  render() {
    return html`
      <span class="badge variant-${this.variant} size-${this.size}">
        ${this.icon ? html`<span class="icon">${this.icon}</span>` : ''}
        <slot></slot>
      </span>
    `;
  }
}

customElements.define('srdb-badge', Badge);
