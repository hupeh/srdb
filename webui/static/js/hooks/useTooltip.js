import { useRef, useEffect } from 'preact/hooks';
import align from 'dom-align';

export function useTooltip() {
    const tooltipRef = useRef(null);
    const hideTimeoutRef = useRef(null);

    useEffect(() => {
        // 创建 tooltip 元素
        const tooltip = document.createElement('div');
        tooltip.className = 'srdb-tooltip';
        tooltip.style.cssText = `
            position: absolute;
            z-index: 1000;
            background: var(--bg-elevated);
            border: 1px solid var(--border-color);
            border-radius: var(--radius-sm);
            padding: 8px 12px;
            font-size: 13px;
            color: var(--text-primary);
            box-shadow: var(--shadow-lg);
            max-width: 300px;
            word-wrap: break-word;
            display: none;
            pointer-events: none;
        `;
        document.body.appendChild(tooltip);
        tooltipRef.current = tooltip;

        return () => {
            if (tooltipRef.current) {
                document.body.removeChild(tooltipRef.current);
            }
            if (hideTimeoutRef.current) {
                clearTimeout(hideTimeoutRef.current);
            }
        };
    }, []);

    const showTooltip = (target, comment) => {
        if (!tooltipRef.current || !comment) return;

        // 清除之前的隐藏定时器
        if (hideTimeoutRef.current) {
            clearTimeout(hideTimeoutRef.current);
            hideTimeoutRef.current = null;
        }

        const tooltip = tooltipRef.current;
        tooltip.textContent = comment;
        tooltip.style.display = 'block';

        // 使用 dom-align 对齐到目标元素下方
        align(tooltip, target, {
            points: ['tc', 'bc'],
            offset: [0, -8],
            overflow: {
                adjustX: true,
                adjustY: true
            }
        });
    };

    const hideTooltip = () => {
        if (!tooltipRef.current) return;

        // 延迟隐藏，避免闪烁
        hideTimeoutRef.current = setTimeout(() => {
            if (tooltipRef.current) {
                tooltipRef.current.style.display = 'none';
            }
        }, 100);
    };

    return { showTooltip, hideTooltip };
}
