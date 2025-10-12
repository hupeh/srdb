import { html } from 'htm/preact';
import { useState } from 'preact/hooks';
import { FieldList } from './FieldList.js';

const styles = {
    tableItem: (isSelected, isExpanded) => ({
        background: 'var(--bg-elevated)',
        border: isSelected ? '1px solid var(--primary)' : '1px solid var(--border-color)',
        borderRadius: 'var(--radius-md)',
        overflow: 'hidden',
        transition: 'var(--transition)',
        boxShadow: isSelected ? '0 0 0 1px var(--primary)' : 'none'
    }),
    tableHeader: (isSelected) => ({
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        paddingLeft: '4px',
        paddingRight: '12px',
        cursor: 'pointer',
        transition: 'var(--transition)',
        background: isSelected ? 'var(--primary-bg)' : 'transparent'
    }),
    tableHeaderLeft: {
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        flex: 1,
        minWidth: 0
    },
    expandIcon: (isExpanded) => ({
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: '32px',
        height: '32px',
        fontSize: '12px',
        transition: 'var(--transition)',
        flexShrink: 0,
        borderRadius: 'var(--radius-sm)',
        cursor: 'pointer',
        color: isExpanded ? 'var(--primary)' : 'var(--text-secondary)',
        transform: isExpanded ? 'rotate(90deg)' : 'rotate(0deg)',
        marginLeft: '-4px'
    }),
    tableName: {
        fontWeight: 500,
        color: 'var(--text-primary)',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis'
    },
    tableCount: {
        fontSize: '12px',
        color: 'var(--text-tertiary)',
        whiteSpace: 'nowrap',
        flexShrink: 0
    }
};

export function TableItem({ table, isSelected, onSelect }) {
    const [isExpanded, setIsExpanded] = useState(false);

    const toggleExpand = (e) => {
        e.stopPropagation();
        setIsExpanded(!isExpanded);
    };

    return html`
        <div
            style=${styles.tableItem(isSelected, isExpanded)}
            onMouseEnter=${(e) => {
                if (!isSelected) {
                    e.currentTarget.style.borderColor = 'var(--border-hover)';
                }
            }}
            onMouseLeave=${(e) => {
                if (!isSelected) {
                    e.currentTarget.style.borderColor = 'var(--border-color)';
                }
            }}
        >
            <!-- 表头 -->
            <div
                style=${styles.tableHeader(isSelected)}
                onClick=${onSelect}
                onMouseEnter=${(e) => {
                    if (!isSelected) {
                        e.currentTarget.style.background = 'var(--bg-hover)';
                    }
                }}
                onMouseLeave=${(e) => {
                    if (!isSelected) {
                        e.currentTarget.style.background = 'transparent';
                    }
                }}
            >
                <div style=${styles.tableHeaderLeft}>
                    <span
                        style=${styles.expandIcon(isExpanded)}
                        onClick=${toggleExpand}
                        onMouseEnter=${(e) => {
                            e.currentTarget.style.background = 'var(--bg-hover)';
                            if (!isExpanded) {
                                e.currentTarget.style.color = 'var(--text-primary)';
                            }
                        }}
                        onMouseLeave=${(e) => {
                            e.currentTarget.style.background = 'transparent';
                            if (!isExpanded) {
                                e.currentTarget.style.color = 'var(--text-secondary)';
                            }
                        }}
                    >
                        ▶
                    </span>
                    <span style=${styles.tableName}>${table.name}</span>
                </div>
                <span style=${styles.tableCount}>
                    ${table.fields?.length || 0} fields
                </span>
            </div>

            <!-- 字段列表 -->
            ${isExpanded && html`
                <${FieldList} fields=${table.fields} />
            `}
        </div>
    `;
}
