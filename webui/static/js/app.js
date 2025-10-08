import './components/app.js';
import './components/table-list.js';
import './components/table-view.js';
import './components/modal-dialog.js';
import './components/theme-toggle.js';
import './components/badge.js';
import './components/field-icon.js';
import './components/data-view.js';
import './components/manifest-view.js';
import './components/page-header.js';

class App {
  constructor() {
    // 等待 srdb-app 组件渲染完成
    this.appContainer = document.querySelector('srdb-app');
    this.modal = document.querySelector('srdb-modal-dialog');
    
    // 等待组件初始化
    if (this.appContainer) {
      // 使用 updateComplete 等待组件渲染完成
      this.appContainer.updateComplete.then(() => {
        this.tableList = this.appContainer.shadowRoot.querySelector('srdb-table-list');
        this.tableView = this.appContainer.shadowRoot.querySelector('srdb-table-view');
        this.pageHeader = this.appContainer.shadowRoot.querySelector('srdb-page-header');
        this.setupEventListeners();
      });
    } else {
      // 如果组件还未定义，等待它被定义
      customElements.whenDefined('srdb-app').then(() => {
        this.appContainer = document.querySelector('srdb-app');
        this.appContainer.updateComplete.then(() => {
          this.tableList = this.appContainer.shadowRoot.querySelector('srdb-table-list');
          this.tableView = this.appContainer.shadowRoot.querySelector('srdb-table-view');
          this.pageHeader = this.appContainer.shadowRoot.querySelector('srdb-page-header');
          this.setupEventListeners();
        });
      });
    }
  }

  setupEventListeners() {
    // Listen for table selection
    document.addEventListener('table-selected', (e) => {
      const tableName = e.detail.tableName;
      this.pageHeader.tableName = tableName;
      this.pageHeader.view = 'data';
      this.tableView.tableName = tableName;
      this.tableView.view = 'data';
      this.tableView.page = 1;
    });

    // Listen for view change from page-header
    document.addEventListener('view-changed', (e) => {
      this.tableView.view = e.detail.view;
    });

    // Listen for refresh request from page-header
    document.addEventListener('refresh-view', (e) => {
      this.tableView.loadData();
    });

    // Listen for row detail request
    document.addEventListener('show-row-detail', async (e) => {
      const { tableName, seq } = e.detail;
      await this.showRowDetail(tableName, seq);
    });

    // Listen for cell content request
    document.addEventListener('show-cell-content', (e) => {
      this.showCellContent(e.detail.content);
    });

    // Close modal on backdrop click
    this.modal.addEventListener('click', () => {
      this.modal.open = false;
    });
  }

  async showRowDetail(tableName, seq) {
    try {
      const response = await fetch(`/api/tables/${tableName}/data/${seq}`);
      if (!response.ok) throw new Error('Failed to load row detail');
      
      const data = await response.json();
      const content = JSON.stringify(data, null, 2);
      
      this.modal.title = `Row Detail - Seq: ${seq}`;
      this.modal.content = content;
      this.modal.open = true;
    } catch (error) {
      console.error('Error loading row detail:', error);
      this.modal.title = 'Error';
      this.modal.content = `Failed to load row detail: ${error.message}`;
      this.modal.open = true;
    }
  }

  showCellContent(content) {
    this.modal.title = 'Cell Content';
    this.modal.content = String(content);
    this.modal.open = true;
  }
}

// Initialize app when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', () => new App());
} else {
  new App();
}
