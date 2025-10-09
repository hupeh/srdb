import { css } from 'lit';

// 共享的基础样式
export const sharedStyles = css`
  * {
    box-sizing: border-box;
    margin: 0;
    padding: 0;
  }

  /* 自定义滚动条样式 */
  *::-webkit-scrollbar {
    width: 8px;
    height: 8px;
  }

  *::-webkit-scrollbar-track {
    background: transparent;
  }

  *::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.1);
    border-radius: 4px;
  }

  *::-webkit-scrollbar-thumb:hover {
    background: rgba(255, 255, 255, 0.15);
  }

  /* 通用状态样式 */
  .empty {
    text-align: center;
    padding: 60px 20px;
    color: var(--text-secondary);
  }

  .loading {
    text-align: center;
    padding: 60px 20px;
    color: var(--text-secondary);
  }
`;

// CSS 变量（可以在组件中使用，优先使用外部定义的变量）
export const cssVariables = css`
  :host {
    /* 主色调 - 优雅的紫蓝色 */
    --primary: var(--srdb-primary, #6366f1);
    --primary-dark: var(--srdb-primary-dark, #4f46e5);
    --primary-light: var(--srdb-primary-light, #818cf8);
    --primary-bg: var(--srdb-primary-bg, rgba(99, 102, 241, 0.1));

    /* 背景色 */
    --bg-main: var(--srdb-bg-main, #0f0f1a);
    --bg-surface: var(--srdb-bg-surface, #1a1a2e);
    --bg-elevated: var(--srdb-bg-elevated, #222236);
    --bg-hover: var(--srdb-bg-hover, #2a2a3e);

    /* 文字颜色 */
    --text-primary: var(--srdb-text-primary, #ffffff);
    --text-secondary: var(--srdb-text-secondary, #a0a0b0);
    --text-tertiary: var(--srdb-text-tertiary, #6b6b7b);

    /* 边框和分隔线 */
    --border-color: var(--srdb-border-color, rgba(255, 255, 255, 0.1));
    --border-hover: var(--srdb-border-hover, rgba(255, 255, 255, 0.2));

    /* 状态颜色 */
    --success: var(--srdb-success, #10b981);
    --warning: var(--srdb-warning, #f59e0b);
    --danger: var(--srdb-danger, #ef4444);
    --info: var(--srdb-info, #3b82f6);

    /* 阴影 */
    --shadow-sm: var(--srdb-shadow-sm, 0 1px 2px 0 rgba(0, 0, 0, 0.3));
    --shadow-md: var(--srdb-shadow-md, 0 4px 6px -1px rgba(0, 0, 0, 0.4), 0 2px 4px -1px rgba(0, 0, 0, 0.3));
    --shadow-lg: var(--srdb-shadow-lg, 0 10px 15px -3px rgba(0, 0, 0, 0.5), 0 4px 6px -2px rgba(0, 0, 0, 0.3));
    --shadow-xl: var(--srdb-shadow-xl, 0 20px 25px -5px rgba(0, 0, 0, 0.5), 0 10px 10px -5px rgba(0, 0, 0, 0.3));

    /* 圆角 */
    --radius-sm: var(--srdb-radius-sm, 6px);
    --radius-md: var(--srdb-radius-md, 8px);
    --radius-lg: var(--srdb-radius-lg, 12px);
    --radius-xl: var(--srdb-radius-xl, 16px);

    /* 过渡 */
    --transition: var(--srdb-transition, all 0.2s cubic-bezier(0.4, 0, 0.2, 1));
  }
`;
