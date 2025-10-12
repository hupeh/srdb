import { html } from 'htm/preact';
import { useState } from 'preact/hooks';
import { TableCell } from './TableCell.js';

const styles = {
    td: {
        padding: '10px 12px',
        borderBottom: '1px solid var(--border-color)',
        color: 'var(--text-primary)',
        textAlign: 'center'
    },
    iconButton: (isRowHovered, isButtonHovered) => ({
        width: '28px',
        height: '28px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: isButtonHovered ? 'var(--primary)' : 'var(--bg-elevated)',
        color: isButtonHovered ? '#fff' : 'var(--primary)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-sm)',
        cursor: 'pointer',
        fontSize: '14px',
        transition: 'all 0.2s ease',
        opacity: isRowHovered ? 1 : 0,
        pointerEvents: isRowHovered ? 'auto' : 'none'
    })
};

export function TableRow({ row, columns, onViewDetail, onShowPopover, onHidePopover }) {
    const [isRowHovered, setIsRowHovered] = useState(false);
    const [isButtonHovered, setIsButtonHovered] = useState(false);

    return html`
        <tr
            onMouseEnter=${(e) => {
                e.currentTarget.style.background = 'var(--bg-hover)';
                setIsRowHovered(true);
            }}
            onMouseLeave=${(e) => {
                e.currentTarget.style.background = 'transparent';
                setIsRowHovered(false);
            }}
        >
            ${columns.map(col => html`
                <${TableCell}
                    key=${col}
                    value=${row[col]}
                    column=${col}
                    isTruncated=${row[col + '_truncated']}
                    onShowPopover=${onShowPopover}
                    onHidePopover=${onHidePopover}
                />
            `)}
            <td style=${styles.td}>
                <button
                    style=${styles.iconButton(isRowHovered, isButtonHovered)}
                    onClick=${() => onViewDetail(row._seq)}
                    onMouseEnter=${() => setIsButtonHovered(true)}
                    onMouseLeave=${() => setIsButtonHovered(false)}
                    title="查看详情"
                >
                    👁
                </button>
            </td>
        </tr>
    `;
}
