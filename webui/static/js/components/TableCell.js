import { html } from 'htm/preact';

const styles = {
    td: {
        padding: '10px 12px',
        borderBottom: '1px solid var(--border-color)',
        color: 'var(--text-primary)',
        maxWidth: '300px',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap',
        cursor: 'pointer'
    }
};

function formatValue(value, col) {
    if (col === '_time') {
        return formatTime(value);
    }

    if (value === null) return 'null';
    if (value === undefined) return 'undefined';

    if (typeof value === 'object') {
        try {
            return JSON.stringify(value, null, 2);
        } catch (e) {
            return '[Object]';
        }
    }

    return String(value);
}

function formatTime(nanoTime) {
    if (!nanoTime) return '';
    const date = new Date(nanoTime / 1000000);
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    const seconds = String(date.getSeconds()).padStart(2, '0');
    return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
}

export function TableCell({ value, column, isTruncated, onShowPopover, onHidePopover }) {
    const formattedValue = formatValue(value, column);

    const handleMouseEnter = (e) => {
        if (onShowPopover) {
            onShowPopover(e, formattedValue);
        }
    };

    const handleMouseLeave = () => {
        if (onHidePopover) {
            onHidePopover();
        }
    };

    return html`
        <td
            style=${styles.td}
            onMouseEnter=${handleMouseEnter}
            onMouseLeave=${handleMouseLeave}
        >
            ${formattedValue}
            ${isTruncated && html`<span style=${{ marginLeft: '4px', fontSize: '10px' }}>✂️</span>`}
        </td>
    `;
}
