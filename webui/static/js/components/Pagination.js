import { html } from 'htm/preact';
import { PageJumper } from './PageJumper.js';

const styles = {
    pagination: {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '12px',
        background: 'var(--bg-surface)',
        borderRadius: 'var(--radius-md)',
        border: '1px solid var(--border-color)',
        position: 'sticky',
        bottom: 0,
        zIndex: 10,
        boxShadow: 'var(--shadow-md)'
    },
    paginationInfo: {
        fontSize: '13px',
        color: 'var(--text-secondary)'
    },
    paginationButtons: {
        display: 'flex',
        gap: '8px',
        alignItems: 'center'
    },
    pageButton: (disabled) => ({
        padding: '6px 12px',
        background: disabled ? 'var(--bg-surface)' : 'var(--bg-elevated)',
        color: disabled ? 'var(--text-tertiary)' : 'var(--text-primary)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-sm)',
        cursor: disabled ? 'not-allowed' : 'pointer',
        fontSize: '13px',
        transition: 'var(--transition)',
        fontWeight: 500
    }),
    pageSizeSelect: {
        padding: '6px 10px',
        background: 'var(--bg-elevated)',
        color: 'var(--text-primary)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-sm)',
        fontSize: '13px',
        cursor: 'pointer'
    }
};

export function Pagination({
    page,
    pageSize,
    totalRows,
    onPageChange,
    onPageSizeChange,
    onJumpToPage
}) {
    const totalPages = Math.ceil(totalRows / pageSize);
    const currentPage = page + 1;
    const startRow = page * pageSize + 1;
    const endRow = Math.min((page + 1) * pageSize, totalRows);

    const handlePrevPage = () => {
        if (page > 0) {
            onPageChange(page - 1);
        }
    };

    const handleNextPage = () => {
        if (page < totalPages - 1) {
            onPageChange(page + 1);
        }
    };

    const handlePageSizeChange = (e) => {
        onPageSizeChange(Number(e.target.value));
    };

    return html`
        <div style=${styles.pagination}>
            <div style=${styles.paginationInfo}>
                显示 ${startRow}-${endRow} / 共 ${totalRows} 行
            </div>
            <div style=${styles.paginationButtons}>
                <select
                    value=${pageSize}
                    onChange=${handlePageSizeChange}
                    style=${styles.pageSizeSelect}
                >
                    <option value="10">10 / 页</option>
                    <option value="20">20 / 页</option>
                    <option value="50">50 / 页</option>
                    <option value="100">100 / 页</option>
                </select>
                <button
                    style=${styles.pageButton(page === 0)}
                    onClick=${handlePrevPage}
                    disabled=${page === 0}
                    onMouseEnter=${(e) => !e.target.disabled && (e.target.style.background = 'var(--bg-hover)')}
                    onMouseLeave=${(e) => !e.target.disabled && (e.target.style.background = 'var(--bg-elevated)')}
                >
                    上一页
                </button>
                <span style=${{ fontSize: '13px', color: 'var(--text-secondary)' }}>
                    ${currentPage} / ${totalPages}
                </span>
                <${PageJumper}
                    totalPages=${totalPages}
                    onJump=${onJumpToPage}
                />
                <button
                    style=${styles.pageButton(page >= totalPages - 1)}
                    onClick=${handleNextPage}
                    disabled=${page >= totalPages - 1}
                    onMouseEnter=${(e) => !e.target.disabled && (e.target.style.background = 'var(--bg-hover)')}
                    onMouseLeave=${(e) => !e.target.disabled && (e.target.style.background = 'var(--bg-elevated)')}
                >
                    下一页
                </button>
            </div>
        </div>
    `;
}
