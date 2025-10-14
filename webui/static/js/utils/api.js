// api.js - 统一的 API 服务层

/**
 * 获取 API 基础路径
 * 从全局变量 window.API_BASE 读取，由服务端渲染时注入
 */
function getBasePath() {
    return window.API_BASE || '';
}

/**
 * 构建完整的 API URL
 * @param {string} path - API 路径，如 '/tables' 或 '/tables/users/schema'
 * @returns {string} 完整 URL
 */
function buildApiUrl(path) {
    const basePath = getBasePath();
    const normalizedPath = path.startsWith('/') ? path : '/' + path;
    const fullPath = basePath ? `${basePath}/api${normalizedPath}` : `/api${normalizedPath}`;
    console.log('[API] Request:', fullPath);
    return fullPath;
}

/**
 * 统一的 fetch 封装
 * @param {string} path - API 路径
 * @param {object} options - fetch 选项
 * @returns {Promise<Response>}
 */
async function apiFetch(path, options = {}) {
    const url = buildApiUrl(path);
    const response = await fetch(url, {
        ...options,
        headers: {
            'Content-Type': 'application/json',
            ...options.headers
        }
    });

    if (!response.ok) {
        console.error('[API] Error:', response.status, url);
    }

    return response;
}

// ============ 表相关 API ============

/**
 * 获取所有表列表
 * @returns {Promise<{tables: Array}>}
 */
export async function getTables() {
    const response = await apiFetch('/tables');
    return response.json();
}

/**
 * 获取表的 Schema
 * @param {string} tableName - 表名
 * @returns {Promise<{name: string, fields: Array}>}
 */
export async function getTableSchema(tableName) {
    const response = await apiFetch(`/tables/${tableName}/schema`);
    return response.json();
}

/**
 * 获取表数据（分页）
 * @param {string} tableName - 表名
 * @param {object} params - 查询参数
 * @param {number} params.limit - 每页数量
 * @param {number} params.offset - 偏移量
 * @param {string} params.select - 选择的字段（逗号分隔）
 * @returns {Promise<{data: Array, totalRows: number}>}
 */
export async function getTableData(tableName, params = {}) {
    const { limit = 20, offset = 0, select } = params;

    const queryParams = new URLSearchParams();
    queryParams.append('limit', limit);
    queryParams.append('offset', offset);
    if (select) {
        queryParams.append('select', select);
    }

    const response = await apiFetch(`/tables/${tableName}/data?${queryParams}`);
    return response.json();
}

/**
 * 根据序列号获取单条数据
 * @param {string} tableName - 表名
 * @param {number} seq - 序列号
 * @returns {Promise<object>}
 */
export async function getRowBySeq(tableName, seq) {
    const response = await apiFetch(`/tables/${tableName}/data/${seq}`);
    return response.json();
}

/**
 * 获取表的 Manifest 信息
 * @param {string} tableName - 表名
 * @returns {Promise<object>}
 */
export async function getTableManifest(tableName) {
    const response = await apiFetch(`/tables/${tableName}/manifest`);
    return response.json();
}
