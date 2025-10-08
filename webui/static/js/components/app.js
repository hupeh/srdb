import { LitElement, html, css } from 'https://cdn.jsdelivr.net/gh/lit/dist@3/core/lit-core.min.js';
import { sharedStyles, cssVariables } from '../styles/shared-styles.js';

export class AppContainer extends LitElement {
  static properties = {
    mobileMenuOpen: { type: Boolean }
  };

  constructor() {
    super();
    this.mobileMenuOpen = false;
  }

  toggleMobileMenu() {
    this.mobileMenuOpen = !this.mobileMenuOpen;
  }

  static styles = [
    sharedStyles,
    cssVariables,
    css`
      :host {
        display: block;
        height: 100vh;
      }

      .container {
        display: flex;
        height: 100%;
        overflow: hidden;
      }

      .sidebar {
        width: 280px;
        background: var(--bg-surface);
        border-right: 1px solid var(--border-color);
        overflow-y: auto;
        overflow-x: hidden;
        padding: 16px 12px;
        display: flex;
        flex-direction: column;
        gap: 8px;
      }

      .sidebar::-webkit-scrollbar {
        width: 6px;
      }

      .sidebar-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 4px;
      }

      .sidebar h1 {
        font-size: 18px;
        font-weight: 700;
        letter-spacing: -0.02em;
        background: linear-gradient(135deg, var(--primary-light), var(--primary));
        -webkit-background-clip: text;
        -webkit-text-fill-color: transparent;
      }

      .main {
        flex: 1;
        overflow-y: auto;
        overflow-x: hidden;
        background: var(--bg-main);
        display: flex;
        flex-direction: column;
      }


      /* 移动端遮罩 */
      .mobile-overlay {
        display: none;
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background: rgba(0, 0, 0, 0.5);
        z-index: 999;
      }

      @media (max-width: 768px) {
        .mobile-overlay.show {
          display: block;
        }

        .container {
          flex-direction: column;
        }

        .sidebar {
          position: fixed;
          top: 0;
          left: 0;
          width: 280px;
          height: 100vh;
          border-right: 1px solid var(--border-color);
          border-bottom: none;
          transform: translateX(-100%);
          transition: transform 0.3s ease;
          z-index: 1000;
        }

        .sidebar.open {
          transform: translateX(0);
        }

        .main {
          padding-top: 0;
        }
      }
    `
  ];

  render() {
    return html`
      <!-- 移动端遮罩 -->
      <div class="mobile-overlay ${this.mobileMenuOpen ? 'show' : ''}" @click=${this.toggleMobileMenu}></div>

      <div class="container">
        <!-- 左侧表列表 -->
        <div class="sidebar ${this.mobileMenuOpen ? 'open' : ''}">
          <div class="sidebar-header">
            <h1>SRDB Tables</h1>
            <srdb-theme-toggle></srdb-theme-toggle>
          </div>
          <srdb-table-list @table-selected=${this.toggleMobileMenu}></srdb-table-list>
        </div>

        <!-- 右侧主内容区 -->
        <div class="main">
          <srdb-page-header @toggle-mobile-menu=${this.toggleMobileMenu}></srdb-page-header>
          <srdb-table-view></srdb-table-view>
        </div>
      </div>
    `;
  }
}

customElements.define('srdb-app', AppContainer);
