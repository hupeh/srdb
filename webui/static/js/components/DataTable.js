import { html } from 'htm/preact';
import { useState, useEffect } from 'preact/hooks';
import { RowDetailModal } from './RowDetailModal.js';
import { Pagination } from './Pagination.js';
import { TableRow } from './TableRow.js';
import { useCellPopover } from '../hooks/useCellPopover.js';
import { useTooltip } from '../hooks/useTooltip.js';

const styles = {
    container: {
        display: 'flex',
        flexDirection: 'column',
        gap: '12px',
        position: 'relative'
    },
    loadingBar: {
        position: 'absolute',
        top: 0,
        left: 0,
        right: 0,
        height: '3px',
        background: 'var(--primary)',
        zIndex: 100,
        animation: 'loading-slide 1.5s ease-in-out infinite'
    },
    loadingOverlay: {
        position: 'fixed',
        top: '16px',
        left: '50%',
        transform: 'translateX(-50%)',
        padding: '10px 20px',
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-md)',
        boxShadow: 'var(--shadow-lg)',
        zIndex: 999,
        display: 'flex',
        alignItems: 'center',
        gap: '10px',
        fontSize: '14px',
        color: 'var(--text-primary)',
        fontWeight: 500
    },
    tableWrapper: {
        overflowX: 'auto',
        background: 'var(--bg-surface)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-md)'
    },
    table: {
        width: '100%',
        borderCollapse: 'collapse',
        fontSize: '13px'
    },
    th: {
        background: 'var(--bg-elevated)',
        color: 'var(--text-secondary)',
        fontWeight: 600,
        textAlign: 'left',
        padding: '12px',
        borderBottom: '1px solid var(--border-color)',
        position: 'sticky',
        top: 0,
        zIndex: 1
    }
};

export function DataTable({ schema, tableName, totalRows, selectedColumns = [] }) {
    const [page, setPage] = useState(0);
    const [pageSize, setPageSize] = useState(20);
    const [data, setData] = useState([]);
    const [loading, setLoading] = useState(false);
    const [selectedSeq, setSelectedSeq] = useState(null);

    const { showPopover, hidePopover } = useCellPopover();
    const { showTooltip, hideTooltip } = useTooltip();

    useEffect(() => {
        fetchData();
    }, [tableName, page, pageSize]);

    const fetchData = async () => {
        try {
            setLoading(true);
            const offset = page * pageSize;
            const response = await fetch(`/api/tables/${tableName}/data?limit=${pageSize}&offset=${offset}`);
            if (response.ok) {
                const result = await response.json();
                setData(result.data || []);
            }
        } catch (error) {
            console.error('Failed to fetch data:', error);
        } finally {
            setLoading(false);
        }
    };

    const getColumns = () => {
        let columns = [];

        if (selectedColumns && selectedColumns.length > 0) {
            // 使用选中的列
            columns = [...selectedColumns];
        } else if (schema && schema.fields) {
            // 没有选择时，显示所有字段
            columns = schema.fields.map(f => f.name);
        } else {
            return ['_seq', '_time'];
        }

        // 过滤掉 _seq 和 _time（它们会被固定放到特定位置）
        const filtered = columns.filter(c => c !== '_seq' && c !== '_time');

        // _seq 在开头，其他字段在中间，_time 在倒数第二（Actions 列之前）
        return ['_seq', ...filtered, '_time'];
    };

    const handleViewDetail = (seq) => {
        setSelectedSeq(seq);
    };

    const handlePageSizeChange = (newPageSize) => {
        setPageSize(newPageSize);
        setPage(0);
    };

    const getFieldComment = (fieldName) => {
        if (!schema || !schema.fields) return '';
        const field = schema.fields.find(f => f.name === fieldName);
        return field?.comment || '';
    };

    const columns = getColumns();

    if (!data || data.length === 0) {
        return html`<div class="empty"><p>暂无数据</p></div>`;
    }

    return html`
        <div style=${styles.container}>
            ${loading && html`
                <div style=${styles.loadingOverlay}>
                    <span style=${{ fontSize: '16px' }}>⏳</span>
                    <span>加载中...</span>
                </div>
            `}

            <div style=${styles.tableWrapper}>
                <table style=${styles.table}>
                    <thead>
                        <tr>
                            ${columns.map(col => {
                                const comment = getFieldComment(col);
                                return html`
                                    <th
                                        key=${col}
                                        style=${styles.th}
                                        onMouseEnter=${(e) => comment && showTooltip(e.currentTarget, comment)}
                                        onMouseLeave=${hideTooltip}
                                    >
                                        ${col}
                                    </th>
                                `;
                            })}
                            <th style=${{ ...styles.th, textAlign: 'center' }}>操作</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${data.map((row, idx) => html`
                            <${TableRow}
                                key=${row._seq || idx}
                                row=${row}
                                columns=${columns}
                                onViewDetail=${handleViewDetail}
                                onShowPopover=${showPopover}
                                onHidePopover=${hidePopover}
                            />
                        `)}
                    </tbody>
                </table>
            </div>

            <!-- 分页控件 -->
            <${Pagination}
                page=${page}
                pageSize=${pageSize}
                totalRows=${totalRows}
                onPageChange=${setPage}
                onPageSizeChange=${handlePageSizeChange}
                onJumpToPage=${setPage}
            />

            <!-- 详情模态框 -->
            ${selectedSeq !== null && html`
                <${RowDetailModal}
                    tableName=${tableName}
                    seq=${selectedSeq}
                    onClose=${() => setSelectedSeq(null)}
                />
            `}
        </div>
    `;
}
