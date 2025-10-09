import { LitElement, html, css } from 'lit';
import { sharedStyles, cssVariables } from '~/common/shared-styles.js';
import { tableAPI } from '~/common/api.js';

export class TableList extends LitElement {
  static properties = {
    tables: { type: Array },
    selectedTable: { type: String },
    expandedTables: { type: Set }
  };

  static styles = [
    sharedStyles,
    cssVariables,
    css`
    :host {
      display: block;
      width: 100%;
    }

    .table-item {
      margin-bottom: 8px;
      background: var(--bg-elevated);
      border: 1px solid var(--border-color);
      border-radius: var(--radius-md);
      overflow: hidden;
      transition: var(--transition);
    }

    .table-item:hover {
      border-color: var(--border-hover);
    }

    .table-item.selected {
      border-color: var(--primary);
      box-shadow: 0 0 0 1px var(--primary);
    }

    .table-item.selected .table-header {
      background: var(--primary-bg);
    }

    .table-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 10px 12px;
      cursor: pointer;
      transition: var(--transition);
    }

    .table-header:hover {
      background: var(--bg-hover);
    }

    .table-item.has-expanded .table-header {
      border-bottom-color: var(--border-color);
    }

    .table-header-left {
      display: flex;
      align-items: center;
      gap: 8px;
      flex: 1;
      min-width: 0;
    }

    .expand-icon {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 24px;
      height: 24px;
      font-size: 12px;
      transition: var(--transition);
      flex-shrink: 0;
      border-radius: var(--radius-sm);
      cursor: pointer;
      color: var(--text-secondary);
    }

    .expand-icon:hover {
      background: var(--bg-hover);
      color: var(--text-primary);
    }

    .expand-icon.expanded {
      transform: rotate(90deg);
      color: var(--primary);
    }

    .table-name {
      font-weight: 500;
      color: var(--text-primary);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .table-count {
      font-size: 12px;
      color: var(--text-tertiary);
      white-space: nowrap;
      flex-shrink: 0;
    }

    /* Schema 字段列表 */
    .schema-fields {
      display: none;
      flex-direction: column;
      border-top: 1px solid transparent;
      transition: var(--transition);
      position: relative;
    }

    .schema-fields.expanded {
      display: block;
      border-top-color: var(--border-color);
    }

    /* 共享的垂直线 */
    .schema-fields.expanded::before {
      z-index: 2;
      content: "";
      position: absolute;
      left: 24px;
      top: 0;
      bottom: 24px;
      width: 1px;
      background: var(--border-color);
    }

    .field-item {
      z-index: 1;
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 12px 8px 24px;
      font-size: 12px;
      transition: var(--transition);
      position: relative;
    }

    /* 每个字段的水平线 */
    .field-item::before {
      content: "";
      width: 8px;
      height: 1px;
      background: var(--border-color);
    }

    .field-item:hover {
      background: var(--bg-hover);
    }

    .field-item:last-child {
      padding-bottom: 12px;
    }

    .field-index-icon {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 20px;
      height: 20px;
      font-size: 14px;
      flex-shrink: 0;
    }

    .field-index-icon.indexed {
      color: var(--success);
    }

    .field-index-icon.not-indexed {
      color: var(--text-tertiary);
      opacity: 0.5;
    }

    .field-name {
      font-weight: 500;
      color: var(--text-secondary);
      flex: 1;
    }
    .field-type {
      font-family: 'Courier New', monospace;
    }

    .loading {
      padding: 20px;
    }

    .error {
      text-align: center;
      padding: 20px;
      color: var(--danger);
    }
  `
  ];

  constructor() {
    super();
    this.tables = [];
    this.selectedTable = '';
    this.expandedTables = new Set();
  }

  connectedCallback() {
    super.connectedCallback();
    this.loadTables();
  }

  async loadTables() {
    try {
      this.tables = await tableAPI.list();
    } catch (error) {
      console.error('Error loading tables:', error);
    }
  }

  toggleExpand(tableName, event) {
    event.stopPropagation();
    if (this.expandedTables.has(tableName)) {
      this.expandedTables.delete(tableName);
    } else {
      this.expandedTables.add(tableName);
    }
    this.requestUpdate();
  }

  selectTable(tableName) {
    this.selectedTable = tableName;
    this.dispatchEvent(new CustomEvent('table-selected', {
      detail: { tableName },
      bubbles: true,
      composed: true
    }));
  }

  render() {
    if (this.tables.length === 0) {
      return html`<div class="loading">Loading tables...</div>`;
    }

    return html`
      ${this.tables.map(table => html`
        <div class="table-item ${this.expandedTables.has(table.name) ? 'has-expanded' : ''} ${this.selectedTable === table.name ? 'selected' : ''}">
          <div 
            class="table-header"
            @click=${() => this.selectTable(table.name)}
          >
            <div class="table-header-left">
              <span 
                class="expand-icon ${this.expandedTables.has(table.name) ? 'expanded' : ''}"
                @click=${(e) => this.toggleExpand(table.name, e)}
              >
                ▶
              </span>
              <span class="table-name">${table.name}</span>
            </div>
            <span class="table-count">${table.fields.length} fields</span>
          </div>
          
          <div class="schema-fields ${this.expandedTables.has(table.name) ? 'expanded' : ''}">
            ${table.fields.map(field => html`
              <div class="field-item">
                <srdb-field-icon 
                  ?indexed=${field.indexed}
                  class="field-index-icon"
                  title="${field.indexed ? 'Indexed field (fast)' : 'Not indexed (slow)'}"
                ></srdb-field-icon>
                <span class="field-name">${field.name}</span>
                <srdb-badge variant="primary" class="field-type">
                  ${field.type}
                </srdb-badge>
              </div>
            `)}
          </div>
        </div>
      `)}
    `;
  }
}

customElements.define('srdb-table-list', TableList);
