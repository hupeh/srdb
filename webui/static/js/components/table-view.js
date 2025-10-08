import { LitElement, html, css } from 'https://cdn.jsdelivr.net/gh/lit/dist@3/core/lit-core.min.js';
import { sharedStyles, cssVariables } from '../styles/shared-styles.js';

export class TableView extends LitElement {
  static properties = {
    tableName: { type: String },
    view: { type: String }, // 'data' or 'manifest'
    schema: { type: Object },
    tableData: { type: Object },
    manifestData: { type: Object },
    selectedColumns: { type: Array },
    page: { type: Number },
    pageSize: { type: Number },
    loading: { type: Boolean }
  };

  static styles = [
    sharedStyles,
    cssVariables,
    css`
      :host {
      display: flex;
      flex-direction: column;
      width: 100%;
      flex: 1;
      position: relative;
      overflow: hidden;
    }

    .content-wrapper {
      flex: 1;
      overflow-y: auto;
      padding: 24px;
      padding-bottom: 80px;
    }

    .pagination {
      position: fixed;
      bottom: 0;
      left: 280px;
      right: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 12px;
      padding: 16px 24px;
      background: var(--bg-elevated);
      border-top: 1px solid var(--border-color);
      z-index: 10;
    }

    @media (max-width: 768px) {
      .pagination {
        left: 0;
      }
    }

    .pagination button {
      padding: 8px 16px;
      background: var(--bg-surface);
      color: var(--text-primary);
      border: 1px solid var(--border-color);
      border-radius: var(--radius-sm);
      cursor: pointer;
      transition: var(--transition);
    }

    .pagination button:hover:not(:disabled) {
      background: var(--bg-hover);
      border-color: var(--border-hover);
    }

    .pagination button:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .pagination select,
    .pagination input {
      padding: 8px 12px;
      background: var(--bg-surface);
      color: var(--text-primary);
      border: 1px solid var(--border-color);
      border-radius: var(--radius-sm);
    }

    .pagination input {
      width: 80px;
    }

    .pagination span {
      color: var(--text-primary);
      font-size: 14px;
    }
  `
  ];

  constructor() {
    super();
    this.tableName = '';
    this.view = 'data';
    this.schema = null;
    this.tableData = null;
    this.manifestData = null;
    this.selectedColumns = [];
    this.page = 1;
    this.pageSize = 20;
    this.loading = false;
  }

  updated(changedProperties) {
    if (changedProperties.has('tableName') && this.tableName) {
      // 切换表时重置选中的列
      this.selectedColumns = [];
      this.page = 1;
      this.loadData();
    }
    if (changedProperties.has('view') && this.tableName) {
      this.loadData();
    }
  }

  async loadData() {
    if (!this.tableName) return;

    this.loading = true;
    
    try {
      // Load schema
      const schemaResponse = await fetch(`/api/tables/${this.tableName}/schema`);
      if (!schemaResponse.ok) throw new Error('Failed to load schema');
      this.schema = await schemaResponse.json();
      
      // Initialize selected columns (all by default)
      if (this.schema.fields) {
        this.selectedColumns = this.schema.fields.map(f => f.name);
      }

      if (this.view === 'data') {
        await this.loadTableData();
      } else if (this.view === 'manifest') {
        await this.loadManifestData();
      }
    } catch (error) {
      console.error('Error loading data:', error);
    } finally {
      this.loading = false;
    }
  }

  async loadTableData() {
    const selectParam = this.selectedColumns.join(',');
    const url = `/api/tables/${this.tableName}/data?page=${this.page}&pageSize=${this.pageSize}&select=${selectParam}`;
    
    const response = await fetch(url);
    if (!response.ok) throw new Error('Failed to load table data');
    this.tableData = await response.json();
  }

  async loadManifestData() {
    const response = await fetch(`/api/tables/${this.tableName}/manifest`);
    if (!response.ok) throw new Error('Failed to load manifest data');
    this.manifestData = await response.json();
  }

  switchView(newView) {
    this.view = newView;
  }

  toggleColumn(columnName) {
    const index = this.selectedColumns.indexOf(columnName);
    if (index > -1) {
      this.selectedColumns = this.selectedColumns.filter(c => c !== columnName);
    } else {
      this.selectedColumns = [...this.selectedColumns, columnName];
    }
    this.loadTableData();
  }

  changePage(delta) {
    this.page = Math.max(1, this.page + delta);
    this.loadTableData();
  }

  changePageSize(newSize) {
    this.pageSize = parseInt(newSize);
    this.page = 1;
    this.loadTableData();
  }

  jumpToPage(pageNum) {
    const num = parseInt(pageNum);
    if (num > 0 && this.tableData && num <= this.tableData.totalPages) {
      this.page = num;
      this.loadTableData();
    }
  }

  showRowDetail(seq) {
    this.dispatchEvent(new CustomEvent('show-row-detail', {
      detail: { tableName: this.tableName, seq },
      bubbles: true,
      composed: true
    }));
  }

  toggleLevel(level) {
    const levelCard = this.shadowRoot.querySelector(`[data-level="${level}"]`);
    if (levelCard) {
      const fileList = levelCard.querySelector('.file-list');
      const icon = levelCard.querySelector('.expand-icon');
      if (fileList.classList.contains('expanded')) {
        fileList.classList.remove('expanded');
        icon.style.transform = 'rotate(0deg)';
      } else {
        fileList.classList.add('expanded');
        icon.style.transform = 'rotate(90deg)';
      }
    }
  }

  formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i];
  }

  formatCount(count) {
    if (count >= 1000000) return (count / 1000000).toFixed(1) + 'M';
    if (count >= 1000) return (count / 1000).toFixed(1) + 'K';
    return count.toString();
  }

  render() {
    if (!this.tableName) {
      return html`
        <div class="empty">
          <h2>Select a table to view data</h2>
          <p>Choose a table from the sidebar to get started</p>
        </div>
      `;
    }

    if (this.loading) {
      return html`<div class="loading">Loading...</div>`;
    }

    return html`
      <div class="content-wrapper">
        ${this.view === 'data' ? html`
          <srdb-data-view
            .tableName=${this.tableName}
            .schema=${this.schema}
            .tableData=${this.tableData}
            .selectedColumns=${this.selectedColumns}
            .loading=${this.loading}
            @columns-changed=${(e) => {
              this.selectedColumns = e.detail.columns;
              this.loadTableData();
            }}
            @show-row-detail=${(e) => this.showRowDetail(e.detail.seq)}
          ></srdb-data-view>
        ` : html`
          <srdb-manifest-view
            .manifestData=${this.manifestData}
            .loading=${this.loading}
          ></srdb-manifest-view>
        `}
      </div>
      
      ${this.view === 'data' && this.tableData ? this.renderPagination() : ''}
    `;
  }

  renderPagination() {
    return html`
      <div class="pagination">
        <select @change=${(e) => this.changePageSize(e.target.value)}>
          ${[10, 20, 50, 100].map(size => html`
            <option value="${size}" ?selected=${size === this.pageSize}>
              ${size} / page
            </option>
          `)}
        </select>
        
        <button 
          @click=${() => this.changePage(-1)}
          ?disabled=${this.page <= 1}
        >
          Previous
        </button>
        
        <span>
          Page ${this.page} of ${this.tableData.totalPages} 
          (${this.formatCount(this.tableData.totalRows)} rows)
        </span>
        
        <input 
          type="number" 
          min="1" 
          max="${this.tableData.totalPages}"
          placeholder="Jump to"
          @keydown=${(e) => e.key === 'Enter' && this.jumpToPage(e.target.value)}
        />
        
        <button @click=${(e) => this.jumpToPage(e.target.previousElementSibling.value)}>
          Go
        </button>
        
        <button 
          @click=${() => this.changePage(1)}
          ?disabled=${this.page >= this.tableData.totalPages}
        >
          Next
        </button>
      </div>
    `;
  }

  formatCount(count) {
    if (count >= 1000000) return (count / 1000000).toFixed(1) + 'M';
    if (count >= 1000) return (count / 1000).toFixed(1) + 'K';
    return count.toString();
  }

  changePage(delta) {
    this.page = Math.max(1, this.page + delta);
    this.loadTableData();
  }

  changePageSize(newSize) {
    this.pageSize = parseInt(newSize);
    this.page = 1;
    this.loadTableData();
  }

  jumpToPage(pageNum) {
    const num = parseInt(pageNum);
    if (num > 0 && this.tableData && num <= this.tableData.totalPages) {
      this.page = num;
      this.loadTableData();
    }
  }

  showRowDetail(seq) {
    this.dispatchEvent(new CustomEvent('show-row-detail', {
      detail: { tableName: this.tableName, seq },
      bubbles: true,
      composed: true
    }));
  }

  showCellContent(content) {
    this.dispatchEvent(new CustomEvent('show-cell-content', {
      detail: { content },
      bubbles: true,
      composed: true
    }));
  }
}

customElements.define('srdb-table-view', TableView);
