import { html } from 'htm/preact';

const styles = {
    compactionStats: {
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-lg)',
        padding: '16px'
    },
    compactionTitle: {
        fontSize: '15px',
        fontWeight: 600,
        color: 'var(--text-primary)',
        marginBottom: '12px'
    },
    compactionGrid: {
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))',
        gap: '8px'
    },
    compactionItem: {
        display: 'flex',
        alignItems: 'center',
        padding: '6px 0',
        fontSize: '13px',
        gap: "24px"
    }
};

function formatNumber(num) {
    return num.toLocaleString('zh-CN');
}

export function CompactionStats({ stats }) {
    if (!stats) return null;

    return html`
        <div style=${styles.compactionStats}>
            <div style=${styles.compactionTitle}>Compaction 统计</div>
            <div style=${styles.compactionGrid}>
                ${Object.entries(stats).map(([key, value]) => html`
                    <div key=${key} style=${styles.compactionItem}>
                        <span style=${{ color: 'var(--text-secondary)' }}>${key}</span>
                        <span style=${{ color: 'var(--text-primary)', fontWeight: 500 }}>
                            ${typeof value === 'number' ? formatNumber(value) : value}
                        </span>
                    </div>
                `)}
            </div>
        </div>
    `;
}
