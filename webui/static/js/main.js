import { render } from 'preact';
import { html } from 'htm/preact';
import { App } from './components/App.js';

// 渲染应用
render(html`<${App} />`, document.getElementById('app'));
