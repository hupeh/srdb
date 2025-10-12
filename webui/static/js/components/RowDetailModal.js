import { html } from 'htm/preact';
import { useState, useEffect, useRef } from 'preact/hooks';

const styles = {
    overlay: {
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        background: 'rgba(0, 0, 0, 0.5)',
        backdropFilter: 'blur(4px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 10000,
        padding: '20px'
    },
    modal: {
        background: 'var(--bg-elevated)',
        borderRadius: 'var(--radius-lg)',
        boxShadow: 'var(--shadow-xl)',
        maxWidth: '800px',
        width: '100%',
        maxHeight: '90vh',
        display: 'flex',
        flexDirection: 'column',
        border: '1px solid var(--border-color)'
    },
    header: {
        padding: '20px',
        borderBottom: '1px solid var(--border-color)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between'
    },
    title: {
        fontSize: '18px',
        fontWeight: 600,
        color: 'var(--text-primary)',
        margin: 0
    },
    closeButton: {
        background: 'none',
        border: 'none',
        fontSize: '24px',
        color: 'var(--text-secondary)',
        cursor: 'pointer',
        padding: '4px 8px',
        borderRadius: 'var(--radius-sm)',
        transition: 'var(--transition)',
        lineHeight: 1
    },
    content: {
        padding: '20px',
        overflowY: 'auto',
        flex: 1
    },
    fieldGroup: {
        marginBottom: '16px',
        padding: '12px',
        background: 'var(--bg-surface)',
        borderRadius: 'var(--radius-md)',
        border: '1px solid var(--border-color)'
    },
    fieldLabel: {
        fontSize: '12px',
        fontWeight: 600,
        color: 'var(--text-secondary)',
        textTransform: 'uppercase',
        marginBottom: '4px',
        display: 'flex',
        alignItems: 'center',
        gap: '6px'
    },
    fieldValue: {
        fontSize: '14px',
        color: 'var(--text-primary)',
        wordBreak: 'break-word',
        fontFamily: '"Courier New", monospace',
        whiteSpace: 'pre-wrap',
        lineHeight: 1.5
    },
    metaTag: {
        display: 'inline-block',
        padding: '2px 6px',
        fontSize: '10px',
        fontWeight: 600,
        borderRadius: 'var(--radius-sm)',
        background: 'var(--primary-bg)',
        color: 'var(--primary)',
        marginLeft: '6px'
    }
};

export function RowDetailModal({ tableName, seq, onClose }) {
    const overlayRef = useRef(null);
    const [rowData, setRowData] = useState(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        fetchRowData();
    }, [tableName, seq]);

    // ESC 键关闭
    useEffect(() => {
        const handleKeyDown = (e) => {
            if (e.key === 'Escape') {
                onClose();
            }
        };
        document.addEventListener('keydown', handleKeyDown);
        return () => document.removeEventListener('keydown', handleKeyDown);
    }, [onClose]);

    const fetchRowData = async () => {
        try {
            setLoading(true);
            const response = await fetch(`/api/tables/${tableName}/data/${seq}`);
            if (response.ok) {
                const data = await response.json();
                setRowData(data);
            }
        } catch (error) {
            console.error('Failed to fetch row data:', error);
        } finally {
            setLoading(false);
        }
    };

    const handleOverlayClick = (e) => {
        if (e.target === overlayRef.current) {
            onClose();
        }
    };

    const formatTime = (nanoTime) => {
        if (!nanoTime) return '';
        const date = new Date(nanoTime / 1000000);
        return date.toLocaleString('zh-CN', {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
            hour12: false
        });
    };

    const formatValue = (value, key) => {
        if (value === null) return 'null';
        if (value === undefined) return 'undefined';

        if (key === '_time') {
            return formatTime(value);
        }

        if (typeof value === 'object') {
            try {
                return JSON.stringify(value, null, 2);
            } catch (e) {
                return '[Object]';
            }
        }

        return String(value);
    };

    const renderField = (key, value) => {
        const isMeta = key === '_seq' || key === '_time';

        return html`
            <div key=${key} style=${styles.fieldGroup}>
                <div style=${styles.fieldLabel}>
                    ${key}
                    ${isMeta && html`<span style=${styles.metaTag}>系统字段</span>`}
                </div>
                <div style=${styles.fieldValue}>
                    ${formatValue(value, key)}
                </div>
            </div>
        `;
    };

    return html`
        <div
            ref=${overlayRef}
            style=${styles.overlay}
            onClick=${handleOverlayClick}
        >
            <div style=${styles.modal}>
                <div style=${styles.header}>
                    <h3 style=${styles.title}>记录详情 - ${tableName}</h3>
                    <button
                        style=${styles.closeButton}
                        onClick=${onClose}
                        onMouseEnter=${(e) => e.target.style.background = 'var(--bg-hover)'}
                        onMouseLeave=${(e) => e.target.style.background = 'none'}
                    >
                        ×
                    </button>
                </div>
                <div style=${styles.content}>
                    ${loading && html`
                        <div class="loading" style=${{ textAlign: 'center', padding: '40px' }}>
                            <p>加载中...</p>
                        </div>
                    `}
                    ${!loading && rowData && html`
                        <div>
                            ${Object.entries(rowData).map(([key, value]) => renderField(key, value))}
                        </div>
                    `}
                    ${!loading && !rowData && html`
                        <div class="empty" style=${{ textAlign: 'center', padding: '40px' }}>
                            <p>未找到数据</p>
                        </div>
                    `}
                </div>
            </div>
        </div>
    `;
}
