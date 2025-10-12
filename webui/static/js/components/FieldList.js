import { html } from 'htm/preact';

const styles = {
    schemaFields: {
        borderTop: '1px solid var(--border-color)',
        position: 'relative',
        transition: 'var(--transition)'
    },
    verticalLine: {
        content: '""',
        position: 'absolute',
        left: '16px',
        top: 0,
        bottom: '14px',
        width: '1px',
        background: 'var(--border-color)',
        zIndex: 2
    },
    fieldItem: {
        zIndex: 1,
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        padding: '4px 12px 4px 16px',
        fontSize: '12px',
        transition: 'var(--transition)',
        position: 'relative'
    },
    horizontalLine: {
        width: '8px',
        height: '1px',
        background: 'var(--border-color)'
    },
    fieldIndexIcon: {
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontSize: '12px',
        flexShrink: 0
    },
    fieldName: {
        fontWeight: 500,
        color: 'var(--text-secondary)',
        flex: 1,
        display: 'flex',
        alignItems: 'center',
        gap: '4px'
    },
    fieldType: {
        fontFamily: '"Courier New", monospace',
        fontSize: '11px',
        padding: '2px 6px',
        background: 'var(--primary-bg)',
        color: 'var(--primary)',
        borderRadius: 'var(--radius-sm)',
        fontWeight: 600
    }
};

export function FieldList({ fields }) {
    if (!fields || fields.length === 0) {
        return null;
    }

    return html`
        <div style=${styles.schemaFields}>
            <!-- 垂直连接线 -->
            <div style=${styles.verticalLine}></div>

            ${fields.map((field) => {
                return html`
                    <div
                        key=${field.name}
                        style=${styles.fieldItem}
                        onMouseEnter=${(e) => {
                            e.currentTarget.style.background = 'var(--bg-hover)';
                        }}
                        onMouseLeave=${(e) => {
                            e.currentTarget.style.background = 'transparent';
                        }}
                    >
                        <!-- 水平连接线 -->
                        <div style=${styles.horizontalLine}></div>

                        <!-- 字段名称和索引图标 -->
                        <span style=${styles.fieldName}>
                            ${field.name}
                            ${field.indexed && html`
                                <span
                                    style=${styles.fieldIndexIcon}
                                    title="Indexed field"
                                >
                                    🔍
                                </span>
                            `}
                        </span>

                        <!-- 字段类型 -->
                        <span style=${styles.fieldType}>${field.type}</span>
                    </div>
                `;
            })}
        </div>
    `;
}
