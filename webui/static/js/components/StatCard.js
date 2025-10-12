import { html } from 'htm/preact';

const styles = {
    statCard: {
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-md)',
        padding: '12px',
        transition: 'var(--transition)'
    },
    statLabel: {
        fontSize: '12px',
        color: 'var(--text-secondary)',
        marginBottom: '6px',
        textTransform: 'uppercase',
        fontWeight: 600
    },
    statValue: {
        fontSize: '22px',
        fontWeight: 600,
        color: 'var(--primary)',
        fontFamily: '"Courier New", monospace'
    }
};

export function StatCard({ label, value }) {
    return html`
        <div style=${styles.statCard}>
            <div style=${styles.statLabel}>${label}</div>
            <div style=${styles.statValue}>${value}</div>
        </div>
    `;
}
