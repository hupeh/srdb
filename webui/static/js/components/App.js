import { html } from 'htm/preact';
import { useState, useEffect } from 'preact/hooks';
import { Sidebar } from '~/components/Sidebar.js';
import { TableView } from '~/components/TableView.js';
import { getTables } from '~/utils/api.js';

export function App() {
    const [theme, setTheme] = useState('dark');
    const [tables, setTables] = useState([]);
    const [selectedTable, setSelectedTable] = useState(null);
    const [loading, setLoading] = useState(true);

    // 初始化主题
    useEffect(() => {
        const savedTheme = localStorage.getItem('srdb_theme') || 'dark';
        setTheme(savedTheme);
        if (savedTheme === 'light') {
            document.documentElement.setAttribute('data-theme', 'light');
        }
    }, []);

    // 加载表列表
    useEffect(() => {
        fetchTables();
    }, []);

    const fetchTables = async () => {
        try {
            setLoading(true);
            const data = await getTables();
            setTables(data.tables || []);
            if (data.tables && data.tables.length > 0) {
                setSelectedTable(data.tables[0].name);
            }
        } catch (error) {
            console.error('Failed to fetch tables:', error);
        } finally {
            setLoading(false);
        }
    };

    const toggleTheme = () => {
        const newTheme = theme === 'dark' ? 'light' : 'dark';
        setTheme(newTheme);
        localStorage.setItem('srdb_theme', newTheme);

        if (newTheme === 'light') {
            document.documentElement.setAttribute('data-theme', 'light');
        } else {
            document.documentElement.removeAttribute('data-theme');
        }
    };

    const styles = {
        container: {
            display: 'flex',
            height: '100vh',
            overflow: 'hidden'
        },
        sidebar: {
            width: '280px',
            background: 'var(--bg-surface)',
            borderRight: '1px solid var(--border-color)',
            overflowY: 'auto',
            overflowX: 'hidden',
            padding: '16px 12px',
        },
        sidebarHeader: {
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            marginBottom: '4px',
            position: 'sticky',
            top: '-16px',
            marginTop: '-16px',
            marginInline: '-12px',
            padding: '16px 12px',
            zIndex: 10,
            background: 'var(--bg-surface)',
        },
        sidebarTitle: {
            fontSize: '18px',
            fontWeight: 700,
            letterSpacing: '-0.02em',
            background: 'linear-gradient(135deg, #667eea, #764ba2)',
            WebkitBackgroundClip: 'text',
            WebkitTextFillColor: 'transparent',
            margin: 0
        },
        themeToggle: {
            width: '32px',
            height: '32px',
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-color)',
            borderRadius: 'var(--radius-md)',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '16px',
            transition: 'var(--transition)'
        },
        main: {
            flex: 1,
            overflowY: 'auto',
            overflowX: 'hidden',
            background: 'var(--bg-main)',
            padding: '24px'
        }
    };

    return html`
        <div style=${styles.container}>
            <!-- 左侧侧边栏 -->
            <div style=${styles.sidebar}>
                <div style=${styles.sidebarHeader}>
                    <h1 style=${styles.sidebarTitle}>SRDB Tables</h1>
                    <button
                        style=${styles.themeToggle}
                        onClick=${toggleTheme}
                        onMouseEnter=${(e) => e.target.style.background = 'var(--bg-hover)'}
                        onMouseLeave=${(e) => e.target.style.background = 'var(--bg-elevated)'}
                        title=${theme === 'dark' ? '切换到浅色主题' : '切换到深色主题'}
                    >
                        ${theme === 'dark' ? '☀️' : '🌙'}
                    </button>
                </div>
                <${Sidebar}
                    tables=${tables}
                    selectedTable=${selectedTable}
                    onSelectTable=${setSelectedTable}
                    loading=${loading}
                />
            </div>

            <!-- 右侧主内容区 -->
            <div style=${styles.main}>
                ${!selectedTable && html`
                    <div class="empty">
                        <p>Select a table from the sidebar</p>
                    </div>
                `}

                ${selectedTable && html`
                    <${TableView} tableName=${selectedTable} key=${selectedTable} />
                `}
            </div>
        </div>
    `;
}
