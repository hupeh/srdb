import { html } from 'htm/preact';
import { TableItem } from '~/components/TableItem.js';

export function Sidebar({ tables, selectedTable, onSelectTable, loading }) {
    if (loading) {
        return html`
            <div class="loading">
                <p>加载中...</p>
            </div>
        `;
    }

    if (tables.length === 0) {
        return html`
            <div class="empty">
                <p>暂无数据表</p>
            </div>
        `;
    }

    return html`
        ${tables.map(table => html`
            <${TableItem}
                key=${table.name}
                table=${table}
                isSelected=${selectedTable === table.name}
                onSelect=${() => onSelectTable(table.name)}
            />
        `)}
    `;
}
