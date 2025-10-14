import { html } from 'htm/preact';
import { useState, useEffect } from 'preact/hooks';
import { DataTable } from '~/components/DataTable.js';
import { ColumnSelector } from '~/components/ColumnSelector.js';
import { ManifestModal } from '~/components/ManifestModal.js';
import { useTooltip } from '~/hooks/useTooltip.js';
import { getTableSchema, getTableData } from '~/utils/api.js';

const styles = {
    container: {
        display: 'flex',
        flexDirection: 'column',
        gap: '20px'
    },
    section: {
        display: 'flex',
        flexDirection: 'column',
        gap: '16px'
    },
    sectionTitle: {
        fontSize: '16px',
        fontWeight: 600,
        color: 'var(--text-primary)'
    },
    manifestButton: {
        display: 'flex',
        alignItems: 'center',
        gap: '6px',
        padding: '8px 16px',
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-md)',
        color: 'var(--text-primary)',
        fontSize: '14px',
        fontWeight: 500,
        cursor: 'pointer',
        transition: 'var(--transition)'
    }
};

export function TableView({ tableName }) {
    const [schema, setSchema] = useState(null);
    const [totalRows, setTotalRows] = useState(0);
    const [loading, setLoading] = useState(true);
    const [selectedColumns, setSelectedColumns] = useState([]);
    const [showManifest, setShowManifest] = useState(false);

    const { showTooltip, hideTooltip } = useTooltip();

    useEffect(() => {
        fetchTableInfo();
    }, [tableName]);

    useEffect(() => {
        // 加载保存的列选择
        if (tableName && schema) {
            const saved = loadSelectedColumns();
            if (saved && saved.length > 0) {
                const validColumns = saved.filter(col =>
                    schema.fields.some(field => field.name === col)
                );
                if (validColumns.length > 0) {
                    setSelectedColumns(validColumns);
                }
            }
        }
    }, [tableName, schema]);

    const fetchTableInfo = async () => {
        try {
            setLoading(true);

            // 获取 Schema
            const schemaData = await getTableSchema(tableName);
            setSchema(schemaData);

            // 获取数据行数（通过一次小查询）
            const data = await getTableData(tableName, { limit: 1, offset: 0 });
            setTotalRows(data.totalRows || 0);
        } catch (error) {
            console.error('Failed to fetch table info:', error);
        } finally {
            setLoading(false);
        }
    };

    const toggleColumn = (columnName) => {
        const index = selectedColumns.indexOf(columnName);
        let newSelection;
        if (index > -1) {
            newSelection = selectedColumns.filter(c => c !== columnName);
        } else {
            newSelection = [...selectedColumns, columnName];
        }
        setSelectedColumns(newSelection);
        saveSelectedColumns(newSelection);
    };

    const saveSelectedColumns = (columns) => {
        if (!tableName) return;
        const key = `srdb_columns_${tableName}`;
        localStorage.setItem(key, JSON.stringify(columns));
    };

    const loadSelectedColumns = () => {
        if (!tableName) return null;
        const key = `srdb_columns_${tableName}`;
        const saved = localStorage.getItem(key);
        return saved ? JSON.parse(saved) : null;
    };

    if (loading) {
        return html`<div class="loading"><p>加载中...</p></div>`;
    }

    return html`
        <div style=${styles.container}>
            <div style=${styles.section}>
                <div style=${{ ...styles.sectionTitle, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <div>
                        <span
                            style=${{ cursor: schema?.comment ? 'help' : 'default' }}
                            onMouseEnter=${(e) => schema?.comment && showTooltip(e.currentTarget, schema.comment)}
                            onMouseLeave=${hideTooltip}
                        >
                            ${tableName}
                        </span>
                        <span style=${{ fontSize: '12px', fontWeight: 400, color: 'var(--text-secondary)', marginLeft: '8px' }}>
                            (共 ${formatCount(totalRows)} 行)
                        </span>
                    </div>
                    <div style=${{ display: 'flex', gap: '8px' }}>
                        <button
                            style=${styles.manifestButton}
                            onClick=${() => setShowManifest(true)}
                            onMouseEnter=${(e) => {
                                e.target.style.background = 'var(--bg-hover)';
                                e.target.style.borderColor = 'var(--border-hover)';
                            }}
                            onMouseLeave=${(e) => {
                                e.target.style.background = 'var(--bg-elevated)';
                                e.target.style.borderColor = 'var(--border-color)';
                            }}
                        >
                            📊 Manifest
                        </button>
                        ${schema && html`
                            <${ColumnSelector}
                                fields=${schema.fields}
                                selectedColumns=${selectedColumns}
                                onToggle=${toggleColumn}
                            />
                        `}
                    </div>
                </div>
                <${DataTable}
                    schema=${schema}
                    tableName=${tableName}
                    totalRows=${totalRows}
                    selectedColumns=${selectedColumns}
                />
            </div>

            ${showManifest && html`
                <${ManifestModal}
                    tableName=${tableName}
                    onClose=${() => setShowManifest(false)}
                />
            `}
        </div>
    `;
}

function formatCount(count) {
    if (count >= 1000000) return (count / 1000000).toFixed(1) + 'M';
    if (count >= 1000) return (count / 1000).toFixed(1) + 'K';
    return count.toString();
}
