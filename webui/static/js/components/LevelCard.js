import { html } from 'htm/preact';
import { FileCard } from './FileCard.js';

const styles = {
    levelSection: {
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-lg)',
        marginBottom: '8px',
        overflow: 'hidden'
    },
    levelHeader: {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        cursor: 'pointer',
        padding: '14px 16px',
        transition: 'var(--transition)',
    },
    levelHeaderLeft: {
        display: 'flex',
        alignItems: 'center',
        gap: '12px',
        flex: 1
    },
    expandIcon: (isExpanded) => ({
        fontSize: '12px',
        color: 'var(--text-secondary)',
        transition: 'transform 0.2s ease',
        userSelect: 'none',
        transform: isExpanded ? 'rotate(90deg)' : 'rotate(0deg)'
    }),
    levelTitle: {
        fontSize: '16px',
        fontWeight: 600,
        color: 'var(--text-primary)',
        display: 'flex',
        alignItems: 'center',
        gap: '12px'
    },
    levelBadge: (level) => {
        const colors = ['#667eea', '#764ba2', '#f093fb', '#4facfe'];
        const bgColor = colors[level] || '#667eea';
        return {
            background: bgColor,
            color: '#fff',
            padding: '4px 10px',
            borderRadius: 'var(--radius-sm)',
            fontSize: '12px',
            fontWeight: 600,
            fontFamily: '"Courier New", monospace'
        };
    },
    levelStats: {
        display: 'flex',
        gap: '20px',
        fontSize: '13px',
        color: 'var(--text-secondary)'
    },
    scoreBadge: (score) => ({
        padding: '4px 8px',
        borderRadius: 'var(--radius-sm)',
        fontSize: '12px',
        fontWeight: 600,
        fontFamily: '"Courier New", monospace',
        background: score > 1 ? '#fed7d7' : score > 0.8 ? '#feebc8' : '#c6f6d5',
        color: score > 1 ? '#c53030' : score > 0.8 ? '#c05621' : '#22543d'
    }),
    filesContainer: (isExpanded) => ({
        display: isExpanded ? 'block' : 'none',
        padding: '12px',
        background: 'var(--bg-surface)',
        borderTop: '1px solid var(--border-color)',
        borderRadius: '0 0 var(--radius-lg) var(--radius-lg)'
    }),
    filesGrid: {
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
        gap: '8px'
    }
};

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i];
}

export function LevelCard({ level, isExpanded, onToggle }) {
    return html`
        <div style=${styles.levelSection}>
            <div
                style=${styles.levelHeader}
                onClick=${onToggle}
                onMouseEnter=${(e) => {
                    e.currentTarget.style.background = 'var(--bg-hover)';
                }}
                onMouseLeave=${(e) => {
                    e.currentTarget.style.background = 'transparent';
                }}
            >
                <div style=${styles.levelHeaderLeft}>
                    <span style=${styles.expandIcon(isExpanded)}>▶</span>
                    <div style=${styles.levelTitle}>
                        <span style=${styles.levelBadge(level.level)}>L${level.level}</span>
                        <span>Level ${level.level}</span>
                    </div>
                </div>
                <div style=${styles.levelStats}>
                    <span>文件: ${level.file_count}</span>
                    <span>大小: ${formatBytes(level.total_size)}</span>
                    ${level.score > 0 && html`
                        <span style=${styles.scoreBadge(level.score)}>
                            Score: ${(level.score * 100).toFixed(0)}%
                        </span>
                    `}
                </div>
            </div>

            ${level.files && level.files.length > 0 && html`
                <div style=${styles.filesContainer(isExpanded)}>
                    <div style=${styles.filesGrid}>
                        ${level.files.map(file => html`
                            <${FileCard} key=${file.file_number} file=${file} />
                        `)}
                    </div>
                </div>
            `}

            ${(!level.files || level.files.length === 0) && html`
                <div class="empty" style=${{ padding: '20px', textAlign: 'center' }}>
                    <p style=${{ color: 'var(--text-tertiary)', fontSize: '13px' }}>此层级暂无文件</p>
                </div>
            `}
        </div>
    `;
}
