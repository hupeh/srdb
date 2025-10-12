import { html } from 'htm/preact';
import { useState, useEffect } from 'preact/hooks';
import { LevelCard } from './LevelCard.js';
import { StatCard } from './StatCard.js';
import { CompactionStats } from './CompactionStats.js';

const styles = {
    container: {
        display: 'flex',
        flexDirection: 'column',
        gap: '12px'
    },
    statsGrid: {
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
        gap: '8px'
    }
};

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i];
}

function formatNumber(num) {
    return num.toLocaleString('zh-CN');
}

export function ManifestView({ tableName }) {
    const [manifest, setManifest] = useState(null);
    const [loading, setLoading] = useState(true);
    const [expandedLevels, setExpandedLevels] = useState(new Set());

    useEffect(() => {
        fetchManifest();
        const interval = setInterval(fetchManifest, 5000); // 每5秒刷新
        return () => clearInterval(interval);
    }, [tableName]);

    const fetchManifest = async () => {
        try {
            const response = await fetch(`/api/tables/${tableName}/manifest`);
            if (response.ok) {
                const data = await response.json();
                setManifest(data);
            }
        } catch (error) {
            console.error('Failed to fetch manifest:', error);
        } finally {
            setLoading(false);
        }
    };

    const toggleLevel = (levelNum) => {
        const newExpanded = new Set(expandedLevels);
        if (newExpanded.has(levelNum)) {
            newExpanded.delete(levelNum);
        } else {
            newExpanded.add(levelNum);
        }
        setExpandedLevels(newExpanded);
    };

    if (loading) {
        return html`<div class="loading"><p>加载中...</p></div>`;
    }

    if (!manifest) {
        return html`<div class="empty"><p>无法加载 Manifest 数据</p></div>`;
    }

    const totalFiles = manifest.levels?.reduce((sum, level) => sum + level.file_count, 0) || 0;
    const totalSize = manifest.levels?.reduce((sum, level) => sum + level.total_size, 0) || 0;

    return html`
        <div style=${styles.container}>
            <!-- 统计信息 -->
            <div style=${styles.statsGrid}>
                <${StatCard} label="总文件数" value=${formatNumber(totalFiles)} />
                <${StatCard} label="总大小" value=${formatBytes(totalSize)} />
                <${StatCard} label="下一个文件号" value=${formatNumber(manifest.next_file_number || 0)} />
                <${StatCard} label="最后序列号" value=${formatNumber(manifest.last_sequence || 0)} />
            </div>

            <!-- Compaction 统计 -->
            <${CompactionStats} stats=${manifest.compaction_stats} />

            <!-- 各层级详情 -->
            ${manifest.levels?.filter(level => level.file_count > 0).map(level => html`
                <${LevelCard}
                    key=${level.level}
                    level=${level}
                    isExpanded=${expandedLevels.has(level.level)}
                    onToggle=${() => toggleLevel(level.level)}
                />
            `)}
        </div>
    `;
}
