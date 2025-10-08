import { LitElement, html, css } from 'https://cdn.jsdelivr.net/gh/lit/dist@3/core/lit-core.min.js';
import { sharedStyles } from '../styles/shared-styles.js';

export class ThemeToggle extends LitElement {
  static properties = {
    theme: { type: String }
  };

  static styles = [
    sharedStyles,
    css`
      :host {
        display: inline-block;
      }

      .theme-toggle {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 8px 12px;
        background: var(--bg-elevated);
        border: 1px solid var(--border-color);
        border-radius: var(--radius-md);
        cursor: pointer;
        transition: var(--transition);
        font-size: 14px;
        color: var(--text-primary);
      }

      .theme-toggle:hover {
        background: var(--bg-hover);
        border-color: var(--border-hover);
      }

      .icon {
        font-size: 18px;
        display: flex;
        align-items: center;
      }

      .label {
        font-weight: 500;
      }
    `
  ];

  constructor() {
    super();
    // 从 localStorage 读取主题，默认为 dark
    this.theme = localStorage.getItem('srdb-theme') || 'dark';
    this.applyTheme();
  }

  toggleTheme() {
    this.theme = this.theme === 'dark' ? 'light' : 'dark';
    localStorage.setItem('srdb-theme', this.theme);
    this.applyTheme();
    
    // 触发主题变化事件
    this.dispatchEvent(new CustomEvent('theme-changed', {
      detail: { theme: this.theme },
      bubbles: true,
      composed: true
    }));
  }

  applyTheme() {
    const root = document.documentElement;
    
    if (this.theme === 'light') {
      // 浅色主题
      root.style.setProperty('--srdb-bg-main', '#ffffff');
      root.style.setProperty('--srdb-bg-surface', '#f5f5f5');
      root.style.setProperty('--srdb-bg-elevated', '#e5e5e5');
      root.style.setProperty('--srdb-bg-hover', '#d4d4d4');
      
      root.style.setProperty('--srdb-text-primary', '#1a1a1a');
      root.style.setProperty('--srdb-text-secondary', '#666666');
      root.style.setProperty('--srdb-text-tertiary', '#999999');
      
      root.style.setProperty('--srdb-border-color', 'rgba(0, 0, 0, 0.1)');
      root.style.setProperty('--srdb-border-hover', 'rgba(0, 0, 0, 0.2)');
      
      root.style.setProperty('--srdb-shadow-sm', '0 1px 2px 0 rgba(0, 0, 0, 0.05)');
      root.style.setProperty('--srdb-shadow-md', '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)');
      root.style.setProperty('--srdb-shadow-lg', '0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05)');
      root.style.setProperty('--srdb-shadow-xl', '0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04)');
    } else {
      // 深色主题（默认值）
      root.style.setProperty('--srdb-bg-main', '#0f0f1a');
      root.style.setProperty('--srdb-bg-surface', '#1a1a2e');
      root.style.setProperty('--srdb-bg-elevated', '#222236');
      root.style.setProperty('--srdb-bg-hover', '#2a2a3e');
      
      root.style.setProperty('--srdb-text-primary', '#ffffff');
      root.style.setProperty('--srdb-text-secondary', '#a0a0b0');
      root.style.setProperty('--srdb-text-tertiary', '#6b6b7b');
      
      root.style.setProperty('--srdb-border-color', 'rgba(255, 255, 255, 0.1)');
      root.style.setProperty('--srdb-border-hover', 'rgba(255, 255, 255, 0.2)');
      
      root.style.setProperty('--srdb-shadow-sm', '0 1px 2px 0 rgba(0, 0, 0, 0.3)');
      root.style.setProperty('--srdb-shadow-md', '0 4px 6px -1px rgba(0, 0, 0, 0.4), 0 2px 4px -1px rgba(0, 0, 0, 0.3)');
      root.style.setProperty('--srdb-shadow-lg', '0 10px 15px -3px rgba(0, 0, 0, 0.5), 0 4px 6px -2px rgba(0, 0, 0, 0.3)');
      root.style.setProperty('--srdb-shadow-xl', '0 20px 25px -5px rgba(0, 0, 0, 0.5), 0 10px 10px -5px rgba(0, 0, 0, 0.3)');
    }
  }

  render() {
    return html`
      <button class="theme-toggle" @click=${this.toggleTheme}>
        <span class="icon">
          ${this.theme === 'dark' ? '🌙' : '☀️'}
        </span>
        <span class="label">
          ${this.theme === 'dark' ? 'Dark' : 'Light'}
        </span>
      </button>
    `;
  }
}

customElements.define('srdb-theme-toggle', ThemeToggle);
