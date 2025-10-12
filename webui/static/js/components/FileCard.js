import { html } from 'htm/preact';

const styles = {
    fileCard: {
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-sm)',
        padding: '10px',
        transition: 'var(--transition)'
    },
    fileName: {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        marginBottom: '6px'
    },
    fileNameText: {
        fontFamily: '"Courier New", monospace',
        fontSize: '13px',
        color: 'var(--text-primary)',
        fontWeight: 500
    },
    fileLevelBadge: {
        fontSize: '11px',
        padding: '2px 8px',
        background: 'var(--primary-bg)',
        color: 'var(--primary)',
        borderRadius: 'var(--radius-sm)',
        fontWeight: 600,
        fontFamily: '"Courier New", monospace'
    },
    fileDetail: {
        display: 'flex',
        flexDirection: 'column',
        gap: '3px',
        fontSize: '12px',
        color: 'var(--text-secondary)'
    },
    fileDetailRow: {
        display: 'flex',
        justifyContent: 'space-between'
    }
};

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i];
}

function formatNumber(num) {
    return num.toLocaleString('zh-CN');
}

export function FileCard({ file }) {
    return html`
        <div
            style=${styles.fileCard}
            onMouseEnter=${(e) => {
                e.currentTarget.style.borderColor = 'var(--primary)';
                e.currentTarget.style.boxShadow = '0 2px 8px rgba(0, 0, 0, 0.1)';
            }}
            onMouseLeave=${(e) => {
                e.currentTarget.style.borderColor = 'var(--border-color)';
                e.currentTarget.style.boxShadow = 'none';
            }}
        >
            <div style=${styles.fileName}>
                <span style=${styles.fileNameText}>${String(file.file_number).padStart(6, '0')}.sst</span>
                <span style=${styles.fileLevelBadge}>L${file.level}</span>
            </div>
            <div style=${styles.fileDetail}>
                <div style=${styles.fileDetailRow}>
                    <span>Size:</span>
                    <span>${formatBytes(file.file_size)}</span>
                </div>
                <div style=${styles.fileDetailRow}>
                    <span>Rows:</span>
                    <span>${formatNumber(file.row_count)}</span>
                </div>
                <div style=${styles.fileDetailRow}>
                    <span>Seq Range:</span>
                    <span>${formatNumber(file.min_key)} - ${formatNumber(file.max_key)}</span>
                </div>
            </div>
        </div>
    `;
}
