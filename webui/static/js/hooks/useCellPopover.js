import { useEffect, useRef } from 'preact/hooks';
import domAlign from 'dom-align';

export function useCellPopover() {
    const popoverRef = useRef(null);
    const hideTimeoutRef = useRef(null);
    const targetCellRef = useRef(null);

    useEffect(() => {
        // 创建 popover 元素
        const popover = document.createElement('div');
        popover.className = 'srdb-popover';
        updatePopoverStyles(popover);
        document.body.appendChild(popover);
        popoverRef.current = popover;

        // 添加样式
        addPopoverStyles();

        // 监听主题变化
        const observer = new MutationObserver(() => {
            updatePopoverStyles(popover);
        });
        observer.observe(document.documentElement, {
            attributes: true,
            attributeFilter: ['data-theme']
        });

        // 清理
        return () => {
            if (popover) {
                popover.remove();
            }
            observer.disconnect();
        };
    }, []);

    const updatePopoverStyles = (popover) => {
        const rootStyles = getComputedStyle(document.documentElement);
        const bgElevated = rootStyles.getPropertyValue('--bg-elevated').trim();
        const textPrimary = rootStyles.getPropertyValue('--text-primary').trim();
        const borderColor = rootStyles.getPropertyValue('--border-color').trim();
        const shadowMd = rootStyles.getPropertyValue('--shadow-md').trim();

        popover.style.cssText = `
            position: fixed;
            z-index: 9999;
            background: ${bgElevated};
            border: 1px solid ${borderColor};
            border-radius: 8px;
            box-shadow: ${shadowMd};
            padding: 12px;
            max-width: 500px;
            max-height: 400px;
            overflow: auto;
            font-size: 13px;
            color: ${textPrimary};
            white-space: pre-wrap;
            word-break: break-word;
            font-family: 'Courier New', monospace;
            opacity: 0;
            transition: opacity 0.15s ease-in-out, background 0.3s ease, color 0.3s ease;
            display: none;
            pointer-events: auto;
        `;
    };

    const addPopoverStyles = () => {
        if (document.getElementById('srdb-popover-styles')) return;

        const style = document.createElement('style');
        style.id = 'srdb-popover-styles';
        style.textContent = `
            .srdb-popover::-webkit-scrollbar {
                width: 8px;
                height: 8px;
            }
            .srdb-popover::-webkit-scrollbar-track {
                background: var(--bg-surface);
                border-radius: 4px;
            }
            .srdb-popover::-webkit-scrollbar-thumb {
                background: var(--border-color);
                border-radius: 4px;
            }
            .srdb-popover::-webkit-scrollbar-thumb:hover {
                background: var(--border-hover);
            }
        `;
        document.head.appendChild(style);
    };

    const showPopover = (e, content) => {
        if (!popoverRef.current) return;

        // 清除隐藏定时器
        if (hideTimeoutRef.current) {
            clearTimeout(hideTimeoutRef.current);
            hideTimeoutRef.current = null;
        }

        // 只在内容较长时显示
        if (content.length < 50) {
            return;
        }

        // 更新 popover
        popoverRef.current.textContent = content;
        popoverRef.current.style.display = 'block';
        targetCellRef.current = e.target;

        // 使用 dom-align 定位
        domAlign(popoverRef.current, e.target, {
            points: ['tl', 'tr'],
            offset: [2, 0],
            overflow: { adjustX: true, adjustY: true }
        });

        // 显示动画
        setTimeout(() => {
            if (popoverRef.current) {
                popoverRef.current.style.opacity = '1';
            }
        }, 10);

        // 添加鼠标事件监听
        popoverRef.current.addEventListener('mouseenter', keepPopover);
        popoverRef.current.addEventListener('mouseleave', hidePopover);
    };

    const hidePopover = () => {
        hideTimeoutRef.current = setTimeout(() => {
            if (popoverRef.current) {
                popoverRef.current.style.opacity = '0';
                setTimeout(() => {
                    if (popoverRef.current) {
                        popoverRef.current.style.display = 'none';
                    }
                }, 150);
            }
        }, 300);
    };

    const keepPopover = () => {
        if (hideTimeoutRef.current) {
            clearTimeout(hideTimeoutRef.current);
            hideTimeoutRef.current = null;
        }
    };

    return {
        showPopover,
        hidePopover
    };
}
