import { LitElement, html, css } from 'lit';
import { sharedStyles, cssVariables } from '~/common/shared-styles.js';

export class PageHeader extends LitElement {
  static properties = {
    tableName: { type: String },
    view: { type: String }
  };

  static styles = [
    sharedStyles,
    cssVariables,
    css`
      :host {
        display: block;
        background: var(--bg-surface);
        border-bottom: 1px solid var(--border-color);
        position: sticky;
        top: 0;
        z-index: 10;
      }

      .header-content {
        padding: 16px 24px;
      }

      .header-top {
        display: flex;
        align-items: center;
        gap: 16px;
        margin-bottom: 12px;
      }

      .mobile-menu-btn {
        display: none;
        width: 40px;
        height: 40px;
        background: var(--primary);
        border: none;
        border-radius: var(--radius-md);
        color: white;
        cursor: pointer;
        flex-shrink: 0;
        transition: var(--transition);
      }

      .mobile-menu-btn:hover {
        background: var(--primary-dark);
      }

      .mobile-menu-btn svg {
        width: 20px;
        height: 20px;
      }

      h2 {
        font-size: 24px;
        font-weight: 600;
        margin: 0;
        color: var(--text-primary);
      }

      .empty-state {
        text-align: center;
        padding: 20px;
        color: var(--text-secondary);
      }

      .view-tabs {
        display: flex;
        align-items: center;
        gap: 8px;
        border-bottom: 1px solid var(--border-color);
        margin: 0 -12px;
        padding: 0 24px;
      }

      .refresh-btn {
        margin-left: auto;
        padding: 8px 12px;
        background: transparent;
        border: 1px solid var(--border-color);
        border-radius: var(--radius-sm);
        color: var(--text-secondary);
        cursor: pointer;
        transition: var(--transition);
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 13px;
      }

      .refresh-btn:hover {
        background: var(--bg-hover);
        border-color: var(--border-hover);
        color: var(--text-primary);
      }

      .refresh-btn svg {
        width: 16px;
        height: 16px;
      }

      .view-tab {
        position: relative;
        padding: 16px 20px;
        background: transparent;
        border: none;
        color: var(--text-secondary);
        cursor: pointer;
        font-size: 14px;
        font-weight: 500;
        transition: var(--transition);
      }

      .view-tab:hover {
        color: var(--text-primary);
        /*background: var(--bg-hover);*/
      }

      .view-tab.active {
        color: var(--primary);
      }

      .view-tab::before,
      .view-tab::after {
        position: absolute;
        display: block;
        content: "";
        z-index: 0;
        opacity: 0;
        inset-inline: 8px;
      }

      .view-tab::before {
        inset-block: 8px;
        border-radius: var(--radius-md);
        background: var(--bg-elevated);
      }

      .view-tab::after {
        border-radius: var(--radius-md);
        background: var(--primary);
        bottom: -2px;
        height: 4px;
      }

      .view-tab.active::before,
      .view-tab.active::after,
      .view-tab:hover::before {
        opacity: 1;
      }

      .view-tab span {
        position: relative;
        z-index: 1;
      }

      @media (max-width: 768px) {
        .mobile-menu-btn {
          display: flex;
          align-items: center;
          justify-content: center;
        }

        .header-content {
          padding: 12px 16px;
        }

        h2 {
          font-size: 18px;
          flex: 1;
        }

        .view-tabs {
          margin: 0 -16px;
          padding: 0 16px;
        }

        .view-tab {
          padding: 8px 16px;
          font-size: 13px;
        }
      }
    `
  ];

  constructor() {
    super();
    this.tableName = '';
    this.view = 'data';
  }

  switchView(newView) {
    this.view = newView;
    this.dispatchEvent(new CustomEvent('view-changed', {
      detail: { view: newView },
      bubbles: true,
      composed: true
    }));
  }

  toggleMobileMenu() {
    this.dispatchEvent(new CustomEvent('toggle-mobile-menu', {
      bubbles: true,
      composed: true
    }));
  }

  refreshView() {
    this.dispatchEvent(new CustomEvent('refresh-view', {
      detail: { view: this.view },
      bubbles: true,
      composed: true
    }));
  }

  render() {
    if (!this.tableName) {
      return html`
        <div class="header-content">
          <div class="empty-state">
            <p>Select a table from the sidebar</p>
          </div>
        </div>
      `;
    }

    return html`
      <div class="header-content">
        <div class="header-top">
          <button class="mobile-menu-btn" @click=${this.toggleMobileMenu}>
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="3" y1="12" x2="21" y2="12"></line>
              <line x1="3" y1="6" x2="21" y2="6"></line>
              <line x1="3" y1="18" x2="21" y2="18"></line>
            </svg>
          </button>
          <h2>${this.tableName}</h2>
        </div>
      </div>
      
      <div class="view-tabs">
        <button 
          class="view-tab ${this.view === 'data' ? 'active' : ''}"
          @click=${() => this.switchView('data')}
        >
          <span>Data</span>
        </button>
        <button
          class="view-tab ${this.view === 'manifest' ? 'active' : ''}"
          @click=${() => this.switchView('manifest')}
        >
          <span>Manifest / Storage Layers</span>
        </button>
        <button class="refresh-btn" @click=${this.refreshView} title="Refresh current view">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21.5 2v6h-6M2.5 22v-6h6M2 11.5a10 10 0 0 1 18.8-4.3M22 12.5a10 10 0 0 1-18.8 4.2"/>
          </svg>
          <span>Refresh</span>
        </button>
      </div>
    `;
  }
}

customElements.define('srdb-page-header', PageHeader);
