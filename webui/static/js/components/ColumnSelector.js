import { html } from 'htm/preact';
import { useState, useRef, useEffect } from 'preact/hooks';

const styles = {
    container: {
        position: 'relative',
        display: 'inline-block'
    },
    button: {
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        padding: '8px 16px',
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-md)',
        color: 'var(--text-primary)',
        fontSize: '14px',
        fontWeight: 500,
        cursor: 'pointer',
        transition: 'var(--transition)'
    },
    dropdown: (isOpen) => ({
        position: 'absolute',
        top: 'calc(100% + 4px)',
        right: 0,
        minWidth: '240px',
        maxHeight: '400px',
        overflowY: 'auto',
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-md)',
        boxShadow: 'var(--shadow-lg)',
        zIndex: 100,
        display: isOpen ? 'block' : 'none'
    }),
    menuItem: (isSelected) => ({
        display: 'flex',
        alignItems: 'center',
        gap: '12px',
        padding: '10px 16px',
        cursor: 'pointer',
        transition: 'var(--transition)',
        background: isSelected ? 'var(--primary-bg)' : 'transparent',
        color: 'var(--text-primary)',
        fontSize: '14px'
    }),
    checkbox: (isSelected) => ({
        width: '18px',
        height: '18px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: 0,
        color: isSelected ? 'var(--primary)' : 'transparent'
    }),
    fieldInfo: {
        flex: 1,
        minWidth: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: '8px'
    },
    fieldName: {
        display: 'flex',
        alignItems: 'center',
        gap: '6px',
        flex: 1,
        minWidth: 0,
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap'
    },
    fieldType: {
        fontSize: '10px',
        color: 'var(--primary)',
        background: 'var(--primary-bg)',
        padding: '2px 5px',
        borderRadius: 'var(--radius-sm)',
        fontFamily: '"Courier New", monospace',
        flexShrink: 0
    }
};

export function ColumnSelector({ fields, selectedColumns, onToggle }) {
    const [isOpen, setIsOpen] = useState(false);
    const dropdownRef = useRef(null);
    const buttonRef = useRef(null);

    useEffect(() => {
        const handleClickOutside = (event) => {
            if (
                dropdownRef.current &&
                buttonRef.current &&
                !dropdownRef.current.contains(event.target) &&
                !buttonRef.current.contains(event.target)
            ) {
                setIsOpen(false);
            }
        };

        if (isOpen) {
            document.addEventListener('mousedown', handleClickOutside);
        }

        return () => {
            document.removeEventListener('mousedown', handleClickOutside);
        };
    }, [isOpen]);

    const selectedCount = selectedColumns.length;
    const totalCount = fields?.length || 0;

    return html`
        <div style=${styles.container}>
            <button
                ref=${buttonRef}
                style=${styles.button}
                onClick=${() => setIsOpen(!isOpen)}
                onMouseEnter=${(e) => {
                    e.target.style.background = 'var(--bg-hover)';
                    e.target.style.borderColor = 'var(--border-hover)';
                }}
                onMouseLeave=${(e) => {
                    e.target.style.background = 'var(--bg-elevated)';
                    e.target.style.borderColor = 'var(--border-color)';
                }}
            >
                <span>Columns</span>
                ${selectedCount > 0 && html`
                    <span style=${{
                        background: 'var(--primary)',
                        color: '#fff',
                        padding: '2px 6px',
                        borderRadius: 'var(--radius-sm)',
                        fontSize: '11px',
                        fontWeight: 600
                    }}>
                        ${selectedCount}
                    </span>
                `}
                <span style=${{
                    fontSize: '12px',
                    color: 'var(--text-secondary)',
                    transition: 'transform 0.2s ease',
                    transform: isOpen ? 'rotate(180deg)' : 'rotate(0deg)'
                }}>▼</span>
            </button>

            <div ref=${dropdownRef} style=${styles.dropdown(isOpen)}>
                ${fields?.map(field => {
                    const isSelected = selectedColumns.includes(field.name);
                    return html`
                        <div
                            key=${field.name}
                            style=${styles.menuItem(isSelected)}
                            onClick=${() => onToggle(field.name)}
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
                            <div style=${styles.checkbox(isSelected)}>
                                ${isSelected ? '✓' : ''}
                            </div>
                            <div style=${styles.fieldInfo}>
                                <div style=${styles.fieldName}>
                                    <span>${field.name}</span>
                                    ${field.indexed && html`<span style=${{ fontSize: '12px' }}>🔍</span>`}
                                </div>
                                <div style=${styles.fieldType}>${field.type}</div>
                            </div>
                        </div>
                    `;
                })}
            </div>
        </div>
    `;
}
