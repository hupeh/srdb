/**
 * API 请求管理模块
 * 统一管理所有后端接口请求
 */

const API_BASE = '/api';

/**
 * 通用请求处理函数
 * @param {string} url - 请求 URL
 * @param {RequestInit} options - fetch 选项
 * @returns {Promise<any>}
 */
async function request(url, options = {}) {
  try {
    const response = await fetch(url, {
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
      ...options,
    });

    if (!response.ok) {
      const error = new Error(`HTTP ${response.status}: ${response.statusText}`);
      error.status = response.status;
      error.response = response;
      throw error;
    }

    return await response.json();
  } catch (error) {
    console.error('API request failed:', url, error);
    throw error;
  }
}

/**
 * 表相关 API
 */
export const tableAPI = {
  /**
   * 获取所有表列表
   * @returns {Promise<Array>}
   */
  async list() {
    return request(`${API_BASE}/tables`);
  },

  /**
   * 获取表的 Schema
   * @param {string} tableName - 表名
   * @returns {Promise<Object>}
   */
  async getSchema(tableName) {
    return request(`${API_BASE}/tables/${tableName}/schema`);
  },

  /**
   * 获取表数据（分页）
   * @param {string} tableName - 表名
   * @param {Object} params - 查询参数
   * @param {number} params.page - 页码
   * @param {number} params.pageSize - 每页大小
   * @param {string} params.select - 选择的列（逗号分隔）
   * @returns {Promise<Object>}
   */
  async getData(tableName, { page = 1, pageSize = 20, select = '' } = {}) {
    const params = new URLSearchParams({
      page: page.toString(),
      pageSize: pageSize.toString(),
    });
    
    if (select) {
      params.append('select', select);
    }

    return request(`${API_BASE}/tables/${tableName}/data?${params}`);
  },

  /**
   * 获取单行数据详情
   * @param {string} tableName - 表名
   * @param {number} seq - 序列号
   * @returns {Promise<Object>}
   */
  async getRow(tableName, seq) {
    return request(`${API_BASE}/tables/${tableName}/data/${seq}`);
  },

  /**
   * 获取表的 Manifest 信息
   * @param {string} tableName - 表名
   * @returns {Promise<Object>}
   */
  async getManifest(tableName) {
    return request(`${API_BASE}/tables/${tableName}/manifest`);
  },

  /**
   * 插入数据
   * @param {string} tableName - 表名
   * @param {Object} data - 数据对象
   * @returns {Promise<Object>}
   */
  async insert(tableName, data) {
    return request(`${API_BASE}/tables/${tableName}/data`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  /**
   * 批量插入数据
   * @param {string} tableName - 表名
   * @param {Array<Object>} data - 数据数组
   * @returns {Promise<Object>}
   */
  async batchInsert(tableName, data) {
    return request(`${API_BASE}/tables/${tableName}/data/batch`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  /**
   * 删除表
   * @param {string} tableName - 表名
   * @returns {Promise<Object>}
   */
  async delete(tableName) {
    return request(`${API_BASE}/tables/${tableName}`, {
      method: 'DELETE',
    });
  },

  /**
   * 获取表统计信息
   * @param {string} tableName - 表名
   * @returns {Promise<Object>}
   */
  async getStats(tableName) {
    return request(`${API_BASE}/tables/${tableName}/stats`);
  },
};

/**
 * 数据库相关 API
 */
export const databaseAPI = {
  /**
   * 获取数据库信息
   * @returns {Promise<Object>}
   */
  async getInfo() {
    return request(`${API_BASE}/database/info`);
  },

  /**
   * 获取数据库统计信息
   * @returns {Promise<Object>}
   */
  async getStats() {
    return request(`${API_BASE}/database/stats`);
  },
};

/**
 * 导出默认 API 对象
 */
export default {
  table: tableAPI,
  database: databaseAPI,
};
