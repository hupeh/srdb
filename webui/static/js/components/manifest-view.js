import { LitElement, html, css } from 'lit';
import { sharedStyles, cssVariables } from '~/common/shared-styles.js';

export class ManifestView extends LitElement {
  static properties = {
    manifestData: { type: Object },
    loading: { type: Boolean },
    expandedLevels: { type: Set }
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

      .manifest-stats {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
        gap: 16px;
        margin-bottom: 24px;
      }

      .stat-card {
        padding: 16px;
        background: var(--bg-elevated);
        border: 1px solid var(--border-color);
        border-radius: var(--radius-md);
      }

      .stat-label {
        font-size: 12px;
        color: var(--text-tertiary);
        margin-bottom: 8px;
      }

      .stat-value {
        font-size: 20px;
        font-weight: 600;
        color: var(--text-primary);
      }

      .level-card {
        margin-bottom: 12px;
        background: var(--bg-elevated);
        border: 1px solid var(--border-color);
        border-radius: var(--radius-md);
        overflow: hidden;
      }

      .level-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 16px;
        cursor: pointer;
        transition: var(--transition);
      }

      .level-header:hover {
        background: var(--bg-hover);
      }

      .level-header-left {
        display: flex;
        align-items: center;
        gap: 12px;
        flex: 1;
      }

      .expand-icon {
        font-size: 12px;
        color: var(--text-secondary);
        transition: transform 0.2s ease;
        user-select: none;
      }

      .expand-icon.expanded {
        transform: rotate(90deg);
      }

      .level-title {
        font-size: 16px;
        font-weight: 600;
        color: var(--text-primary);
      }

      .level-stats {
        display: flex;
        gap: 16px;
        font-size: 13px;
        color: var(--text-secondary);
      }

      .level-files {
        padding: 16px;
        background: var(--bg-surface);
        border-top: 1px solid var(--border-color);
        display: none;
      }

      .level-files.expanded {
        display: block;
      }

      .file-list {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
        gap: 12px;
      }

      .file-item {
        padding: 12px;
        background: var(--bg-elevated);
        border: 1px solid var(--border-color);
        border-radius: var(--radius-sm);
        transition: var(--transition);
      }

      .file-item:hover {
        border-color: var(--primary);
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
      }

      .file-name {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 8px;
      }

      .file-name-text {
        font-family: 'Courier New', monospace;
        font-size: 13px;
        color: var(--text-primary);
        font-weight: 500;
      }

      .file-level-badge {
        font-size: 11px;
        padding: 2px 8px;
        background: var(--primary-bg);
        color: var(--primary);
        border-radius: var(--radius-sm);
        font-weight: 600;
        font-family: 'Courier New', monospace;
      }

      .file-detail {
        display: flex;
        flex-direction: column;
        gap: 4px;
        font-size: 12px;
        color: var(--text-secondary);
      }

      .file-detail-row {
        display: flex;
        justify-content: space-between;
      }

      @media (max-width: 768px) {
        .file-list {
          grid-template-columns: 1fr;
        }
      }

      .empty {
        background: var(--bg-elevated);
        border-radius: var(--radius-md);
        border: 1px dashed var(--border-color);
        margin-top: 24px;
      }

      .empty p {
        margin: 0;
      }
    `
  ];

  constructor() {
    super();
    this.manifestData = null;
    this.loading = false;
    this.expandedLevels = new Set();
  }

  toggleLevel(levelNum) {
    if (this.expandedLevels.has(levelNum)) {
      this.expandedLevels.delete(levelNum);
    } else {
      this.expandedLevels.add(levelNum);
    }
    this.requestUpdate();
  }

  formatSize(bytes) {
    if (bytes >= 1024 * 1024 * 1024) return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB';
    if (bytes >= 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(2) + ' MB';
    if (bytes >= 1024) return (bytes / 1024).toFixed(2) + ' KB';
    return bytes + ' B';
  }

  getScoreVariant(score) {
    if (score >= 0.8) return 'danger';    // 高分 = 需要紧急 compaction
    if (score >= 0.5) return 'warning';   // 中分 = 需要关注
    return 'success';                     // 低分 = 健康状态
  }

  render() {
    if (this.loading || !this.manifestData) {
      return html`<div class="loading">Loading manifest...</div>`;
    }

    const totalFiles = this.manifestData.levels.reduce((sum, l) => sum + l.file_count, 0);
    const totalSize = this.manifestData.levels.reduce((sum, l) => sum + l.total_size, 0);
    const totalCompactions = this.manifestData.compaction_stats?.total_compactions || 0;

    return html`
      <h3>LSM-Tree Structure</h3>
      
      <div class="manifest-stats">
        <div class="stat-card">
          <div class="stat-label">Active Levels</div>
          <div class="stat-value">${this.manifestData.levels.filter(l => l.file_count > 0).length}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Total Files</div>
          <div class="stat-value">${totalFiles}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Total Size</div>
          <div class="stat-value">${this.formatSize(totalSize)}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Compactions</div>
          <div class="stat-value">${totalCompactions}</div>
        </div>
      </div>

      ${this.manifestData.levels && this.manifestData.levels.length > 0 
        ? this.manifestData.levels.map(level => this.renderLevelCard(level))
        : html`
          <div class="empty">
            <p>No SSTable files in this table yet.</p>
            <p style="font-size: 14px; margin-top: 8px;">Insert some data to see the LSM-Tree structure.</p>
          </div>
        `
      }
    `;
  }

  renderLevelCard(level) {
    if (level.file_count === 0) return '';

    const isExpanded = this.expandedLevels.has(level.level);

    return html`
      <div class="level-card">
        <div class="level-header" @click=${() => this.toggleLevel(level.level)}>
          <div class="level-header-left">
            <span class="expand-icon ${isExpanded ? 'expanded' : ''}">▶</span>
            <div>
              <div class="level-title">Level ${level.level}</div>
              <div class="level-stats">
                <span>${level.file_count} files</span>
                <span>${this.formatSize(level.total_size)}</span>
                ${level.score !== undefined ? html`
                  <srdb-badge variant="${this.getScoreVariant(level.score)}">
                    Score: ${(level.score * 100).toFixed(0)}%
                  </srdb-badge>
                ` : ''}
              </div>
            </div>
          </div>
        </div>
        
        ${level.files && level.files.length > 0 ? html`
          <div class="level-files ${isExpanded ? 'expanded' : ''}">
            <div class="file-list">
              ${level.files.map(file => html`
                <div class="file-item">
                  <div class="file-name">
                    <span class="file-name-text">${file.file_number}.sst</span>
                    <span class="file-level-badge">L${level.level}</span>
                  </div>
                  <div class="file-detail">
                    <div class="file-detail-row">
                      <span>Size:</span>
                      <span>${this.formatSize(file.file_size)}</span>
                    </div>
                    <div class="file-detail-row">
                      <span>Rows:</span>
                      <span>${file.row_count || 0}</span>
                    </div>
                    <div class="file-detail-row">
                      <span>Seq Range:</span>
                      <span>${file.min_key} - ${file.max_key}</span>
                    </div>
                  </div>
                </div>
              `)}
            </div>
          </div>
        ` : ''}
      </div>
    `;
  }
}

customElements.define('srdb-manifest-view', ManifestView);
