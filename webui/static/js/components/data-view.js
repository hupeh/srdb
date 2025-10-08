import { LitElement, html, css } from 'https://cdn.jsdelivr.net/gh/lit/dist@3/core/lit-core.min.js';
import { sharedStyles, cssVariables } from '../styles/shared-styles.js';

export class DataView extends LitElement {
  static properties = {
    tableName: { type: String },
    schema: { type: Object },
    tableData: { type: Object },
    selectedColumns: { type: Array },
    loading: { type: Boolean }
  };

  static styles = [
    sharedStyles,
    cssVariables,
    css`
      :host {
        display: block;
      }

      h3 {
        font-size: 16px;
        font-weight: 600;
        margin: 20px 0 12px 0;
        color: var(--text-primary);
      }

      .schema-section {
        margin-bottom: 24px;
      }

      .schema-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
        gap: 12px;
      }

      .schema-field-card {
        padding: 12px;
        background: var(--bg-elevated);
        border: 1px solid var(--border-color);
        border-radius: var(--radius-md);
        cursor: pointer;
        transition: var(--transition);
      }

      .schema-field-card:hover {
        background: var(--bg-hover);
      }

      .schema-field-card.selected {
        border-color: var(--primary);
        background: var(--primary-bg);
      }

      .field-item {
        display: flex;
        flex-direction: column;
        gap: 4px;
      }

      .field-item-row {
        display: flex;
        align-items: center;
        gap: 8px;
      }

      .field-index-icon {
        font-size: 14px;
        flex-shrink: 0;
      }

      .field-name {
        font-weight: 500;
        color: var(--text-primary);
        font-size: 13px;
        flex: 1;
      }

      .field-type {
        font-family: 'Courier New', monospace;
      }

      .field-comment {
        color: var(--text-tertiary);
        font-size: 11px;
        margin-top: 4px;
        font-style: italic;
        min-height: 16px;
      }

      .field-comment:empty::before {
        content: "No comment";
        opacity: 0.5;
      }

      .table-wrapper {
        overflow-x: auto;
        background: var(--bg-elevated);
        border-radius: var(--radius-md);
        border: 1px solid var(--border-color);
      }

      .data-table {
        width: 100%;
        border-collapse: collapse;
        font-size: 13px;
      }

      .data-table th {
        background: var(--bg-surface);
        color: var(--text-secondary);
        font-weight: 600;
        text-align: left;
        padding: 12px;
        border-bottom: 1px solid var(--border-color);
        position: sticky;
        top: 0;
        z-index: 1;
      }

      .data-table td {
        padding: 10px 12px;
        border-bottom: 1px solid var(--border-color);
        color: var(--text-primary);
        max-width: 300px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }

      .data-table td:hover {
        white-space: normal;
        word-break: break-all;
      }

      .data-table tr:hover {
        background: var(--bg-hover);
      }

      .truncated-icon {
        margin-left: 4px;
        font-size: 10px;
      }

      .row-detail-btn {
        padding: 4px 12px;
        background: var(--primary);
        color: white;
        border: none;
        border-radius: var(--radius-sm);
        cursor: pointer;
        font-size: 12px;
        transition: var(--transition);
      }

      .row-detail-btn:hover {
        background: var(--primary-dark);
      }
    `
  ];

  constructor() {
    super();
    this.tableName = '';
    this.schema = null;
    this.tableData = null;
    this.selectedColumns = [];
    this.loading = false;
  }

  updated(changedProperties) {
    // 当 tableName 或 schema 改变时，尝试加载保存的列选择
    if ((changedProperties.has('tableName') || changedProperties.has('schema')) && this.tableName && this.schema) {
      const saved = this.loadSelectedColumns();
      if (saved && saved.length > 0) {
        // 验证保存的列是否仍然存在于当前 schema 中
        const validColumns = saved.filter(col => 
          this.schema.fields.some(field => field.name === col)
        );
        if (validColumns.length > 0) {
          this.selectedColumns = validColumns;
        }
      }
    }
  }

  toggleColumn(columnName) {
    const index = this.selectedColumns.indexOf(columnName);
    if (index > -1) {
      this.selectedColumns = this.selectedColumns.filter(c => c !== columnName);
    } else {
      this.selectedColumns = [...this.selectedColumns, columnName];
    }
    
    // 持久化到 localStorage
    this.saveSelectedColumns();
    
    this.dispatchEvent(new CustomEvent('columns-changed', {
      detail: { columns: this.selectedColumns },
      bubbles: true,
      composed: true
    }));
  }

  saveSelectedColumns() {
    if (!this.tableName) return;
    const key = `srdb_columns_${this.tableName}`;
    localStorage.setItem(key, JSON.stringify(this.selectedColumns));
  }

  loadSelectedColumns() {
    if (!this.tableName) return null;
    const key = `srdb_columns_${this.tableName}`;
    const saved = localStorage.getItem(key);
    return saved ? JSON.parse(saved) : null;
  }

  showRowDetail(seq) {
    this.dispatchEvent(new CustomEvent('show-row-detail', {
      detail: { tableName: this.tableName, seq },
      bubbles: true,
      composed: true
    }));
  }

  formatCount(count) {
    if (count >= 1000000) return (count / 1000000).toFixed(1) + 'M';
    if (count >= 1000) return (count / 1000).toFixed(1) + 'K';
    return count.toString();
  }

  formatTime(nanoTime) {
    if (!nanoTime) return '';
    // 将纳秒转换为毫秒
    const date = new Date(nanoTime / 1000000);
    // 格式化为 YYYY-MM-DD HH:mm:ss
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    const seconds = String(date.getSeconds()).padStart(2, '0');
    return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
  }

  getColumns() {
    let columns = [];
    
    if (this.selectedColumns.length > 0) {
      columns = [...this.selectedColumns];
    } else {
      columns = this.schema?.fields?.map(f => f.name) || [];
    }
    
    // 确保系统字段的顺序：_seq 在开头，_time 在倒数第二
    const filtered = columns.filter(c => c !== '_seq' && c !== '_time');
    
    // _seq 放开头
    const result = ['_seq', ...filtered];
    
    // _time 放倒数第二（Actions 列之前）
    result.push('_time');
    
    return result;
  }

  render() {
    if (this.loading || !this.schema || !this.tableData) {
      return html`<div class="loading">Loading data...</div>`;
    }

    const columns = this.getColumns();

    return html`
      ${this.renderSchemaSection()}
      
      <h3>Data (${this.formatCount(this.tableData.totalRows)} rows)</h3>
      
      ${this.tableData.data.length === 0 ? html`
        <div class="empty"><p>No data available</p></div>
      ` : html`
        <div class="table-wrapper">
          <table class="data-table">
            <thead>
              <tr>
                ${columns.map(col => html`<th>${col}</th>`)}
                <th style="text-align: center;">Actions</th>
              </tr>
            </thead>
            <tbody>
              ${this.tableData.data.map(row => html`
                <tr>
                  ${columns.map(col => html`
                    <td>
                      ${col === '_time' ? this.formatTime(row[col]) : row[col]}
                      ${row[col + '_truncated'] ? html`<span class="truncated-icon">✂️</span>` : ''}
                    </td>
                  `)}
                  <td style="text-align: center;">
                    <button 
                      class="row-detail-btn"
                      @click=${() => this.showRowDetail(row._seq)}
                    >
                      Detail
                    </button>
                  </td>
                </tr>
              `)}
            </tbody>
          </table>
        </div>
      `}
    `;
  }

  renderSchemaSection() {
    if (!this.schema || !this.schema.fields) return '';

    return html`
      <div class="schema-section">
        <h3>Schema <span style="font-size: 12px; font-weight: 400; color: var(--text-secondary);">(点击字段卡片选择要显示的列)</span></h3>
        <div class="schema-grid">
          ${this.schema.fields.map(field => html`
            <div 
              class="schema-field-card ${this.selectedColumns.includes(field.name) ? 'selected' : ''}"
              @click=${() => this.toggleColumn(field.name)}
            >
              <div class="field-item">
                <div class="field-item-row">
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
                <div class="field-comment">${field.comment || ''}</div>
              </div>
            </div>
          `)}
        </div>
      </div>
    `;
  }
}

customElements.define('srdb-data-view', DataView);
