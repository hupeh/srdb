import { LitElement, html, css } from 'lit';

export class FieldIcon extends LitElement {
  static properties = {
    indexed: { type: Boolean }
  };
  static styles = css`
    :host {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 20px;
      height: 20px;
    }

    svg {
      width: 16px;
      height: 16px;
    }

    .indexed {
      fill: var(--success);
      color: var(--success);
      opacity: 1;
    }

    .not-indexed {
      fill: var(--text-secondary);
      color: var(--text-secondary);
      opacity: 0.6;
    }
  `;

  constructor() {
    super();
    this.indexed = false;
  }

  render() {
    if (this.indexed) {
      // 闪电图标 - 已索引（快速）
      return html`
        <svg viewBox="0 0 24 24" class="indexed" xmlns="http://www.w3.org/2000/svg" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" fill="currentColor" stroke="none"/>
        </svg>
      `;
    } else {
      // 圆点图标 - 未索引
      return html`
        <svg viewBox="0 0 24 24" class="not-indexed" xmlns="http://www.w3.org/2000/svg">
          <circle cx="12" cy="12" r="4" fill="currentColor"/>
        </svg>
      `;
    }
  }
}

customElements.define('srdb-field-icon', FieldIcon);
