import { html } from 'htm/preact';
import { useEffect } from 'preact/hooks';
import { ManifestView } from '~/components/ManifestView.js';

const styles = {
    overlay: {
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        background: 'rgba(0, 0, 0, 0.6)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 1000,
        padding: '20px'
    },
    modal: {
        background: 'var(--bg-elevated)',
        borderRadius: 'var(--radius-lg)',
        boxShadow: 'var(--shadow-xl)',
        width: '90vw',
        maxWidth: '1200px',
        maxHeight: '85vh',
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden'
    },
    header: {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '20px 24px',
        borderBottom: '1px solid var(--border-color)'
    },
    title: {
        fontSize: '20px',
        fontWeight: 600,
        color: 'var(--text-primary)',
        margin: 0
    },
    closeButton: {
        width: '32px',
        height: '32px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'transparent',
        border: 'none',
        borderRadius: 'var(--radius-sm)',
        cursor: 'pointer',
        fontSize: '20px',
        color: 'var(--text-secondary)',
        transition: 'var(--transition)'
    },
    content: {
        flex: 1,
        overflowY: 'auto',
        padding: '20px 24px'
    }
};

export function ManifestModal({ tableName, onClose }) {
    // ESC 键关闭
    useEffect(() => {
        const handleEscape = (e) => {
            if (e.key === 'Escape') {
                onClose();
            }
        };

        document.addEventListener('keydown', handleEscape);
        return () => document.removeEventListener('keydown', handleEscape);
    }, [onClose]);

    // 阻止背景滚动
    useEffect(() => {
        document.body.style.overflow = 'hidden';
        return () => {
            document.body.style.overflow = '';
        };
    }, []);

    const handleOverlayClick = (e) => {
        if (e.target === e.currentTarget) {
            onClose();
        }
    };

    return html`
        <div style=${styles.overlay} onClick=${handleOverlayClick}>
            <div style=${styles.modal}>
                <div style=${styles.header}>
                    <h2 style=${styles.title}>Manifest - ${tableName}</h2>
                    <button
                        style=${styles.closeButton}
                        onClick=${onClose}
                        onMouseEnter=${(e) => {
                            e.target.style.background = 'var(--bg-hover)';
                            e.target.style.color = 'var(--text-primary)';
                        }}
                        onMouseLeave=${(e) => {
                            e.target.style.background = 'transparent';
                            e.target.style.color = 'var(--text-secondary)';
                        }}
                        title="关闭"
                    >
                        ✕
                    </button>
                </div>
                <div style=${styles.content}>
                    <${ManifestView} tableName=${tableName} />
                </div>
            </div>
        </div>
    `;
}
