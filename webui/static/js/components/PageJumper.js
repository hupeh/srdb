import { html } from 'htm/preact';
import { Fragment } from 'preact';
import { useState } from 'preact/hooks';

const styles = {
    jumpInput: {
        width: '80px',
        padding: '6px 10px',
        background: 'var(--bg-elevated)',
        color: 'var(--text-primary)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-sm)',
        fontSize: '13px'
    },
    jumpButton: {
        padding: '6px 12px',
        background: 'var(--bg-elevated)',
        color: 'var(--text-primary)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-sm)',
        cursor: 'pointer',
        fontSize: '13px',
        transition: 'var(--transition)',
        fontWeight: 500
    }
};

export function PageJumper({ totalPages, onJump }) {
    const [inputValue, setInputValue] = useState('');

    const handleJump = () => {
        const num = parseInt(inputValue);
        if (num >= 1 && num <= totalPages) {
            onJump(num - 1);
            setInputValue('');
        }
    };

    const handleKeyDown = (e) => {
        if (e.key === 'Enter') {
            handleJump();
        }
    };

    return html`
        <${Fragment}>
            <input
                type="number"
                min="1"
                max=${totalPages}
                placeholder="跳转"
                value=${inputValue}
                onInput=${(e) => setInputValue(e.target.value)}
                onKeyDown=${handleKeyDown}
                style=${styles.jumpInput}
            />
            <button
                style=${styles.jumpButton}
                onClick=${handleJump}
                onMouseEnter=${(e) => {
                    e.target.style.background = 'var(--bg-hover)';
                }}
                onMouseLeave=${(e) => {
                    e.target.style.background = 'var(--bg-elevated)';
                }}
            >
                跳转
            </button>
        <//>
    `;
}
