// The harness runs the reload client of `avero dev` against a small document
// model, so a test proves that a reload morphs the page and never navigates.
//
// It runs under node, which is optional. The test skips when node is absent.
// See DX-9.
const fs = require("node:fs");

const TEXT = 3;
const COMMENT = 8;
const ELEMENT = 1;

class Node2 {
    constructor(nodeType, nodeName) {
        this.nodeType = nodeType;
        this.nodeName = nodeName;
        this.childNodes = [];
        this.parentNode = null;
        this.attributesMap = new Map();
        this.nodeValue = null;
    }
    get attributes() {
        return Array.from(this.attributesMap, ([name, value]) => ({ name, value }));
    }
    hasAttribute(name) { return this.attributesMap.has(name); }
    getAttribute(name) { return this.attributesMap.has(name) ? this.attributesMap.get(name) : null; }
    setAttribute(name, value) { this.attributesMap.set(name, value); }
    removeAttribute(name) { this.attributesMap.delete(name); }
    get id() { return this.getAttribute("id") || ""; }
    get tagName() { return this.nodeName; }
    get firstChild() { return this.childNodes[0] || null; }
    get nextSibling() {
        if (!this.parentNode) return null;
        const i = this.parentNode.childNodes.indexOf(this);
        return this.parentNode.childNodes[i + 1] || null;
    }
    appendChild(child) {
        child.parentNode = this;
        this.childNodes.push(child);
        return child;
    }
    insertBefore(child, before) {
        child.parentNode = this;
        const i = before ? this.childNodes.indexOf(before) : this.childNodes.length;
        this.childNodes.splice(i < 0 ? this.childNodes.length : i, 0, child);
        return child;
    }
    remove() {
        if (!this.parentNode) return;
        const i = this.parentNode.childNodes.indexOf(this);
        if (i >= 0) this.parentNode.childNodes.splice(i, 1);
        this.parentNode = null;
    }
    replaceWith(next) {
        if (!this.parentNode) return;
        const i = this.parentNode.childNodes.indexOf(this);
        next.parentNode = this.parentNode;
        this.parentNode.childNodes[i] = next;
    }
    cloneNode(deep) {
        const copy = new Node2(this.nodeType, this.nodeName);
        copy.nodeValue = this.nodeValue;
        copy.attributesMap = new Map(this.attributesMap);
        if (deep) {
            for (const child of this.childNodes) copy.appendChild(child.cloneNode(true));
        }
        return copy;
    }
    querySelectorAll() { return []; }
    get text() {
        if (this.nodeType === TEXT) return this.nodeValue;
        return this.childNodes.map((c) => c.text).join("");
    }
}

// parse reads a small subset of HTML: elements with attributes and text.
function parse(html) {
    const root = new Node2(ELEMENT, "HTML");
    const stack = [root];
    const pattern = /<(\/?)([a-zA-Z0-9]+)((?:\s+[a-zA-Z-]+="[^"]*")*)\s*>|([^<]+)/g;
    let match;
    while ((match = pattern.exec(html)) !== null) {
        const [, closing, name, attrs, text] = match;
        if (text !== undefined) {
            if (text.trim() === "") continue;
            const node = new Node2(TEXT, "#text");
            node.nodeValue = text;
            stack[stack.length - 1].appendChild(node);
            continue;
        }
        if (closing) {
            stack.pop();
            continue;
        }
        const node = new Node2(ELEMENT, name.toUpperCase());
        for (const pair of attrs.matchAll(/([a-zA-Z-]+)="([^"]*)"/g)) {
            node.setAttribute(pair[1], pair[2]);
        }
        stack[stack.length - 1].appendChild(node);
        stack.push(node);
    }
    return root;
}

const first = process.argv[2];
const second = process.argv[3];

const document = parse(fs.readFileSync(first, "utf8"));
document.documentElement = document;
document.activeElement = null;
document.querySelectorAll = () => [];

const state = { scrollY: 120, navigations: 0 };
const window = {
    location: { href: "http://127.0.0.1/page" },
    get scrollY() { return state.scrollY; },
    scrollTo() { state.scrollY = 0; },
};

globalThis.document = document;
globalThis.window = window;
globalThis.Node = { ELEMENT_NODE: ELEMENT, TEXT_NODE: TEXT, COMMENT_NODE: COMMENT };
globalThis.DOMParser = class {
    parseFromString(text) {
        const parsed = parse(text);
        parsed.documentElement = parsed;
        return parsed;
    }
};
globalThis.fetch = async () => ({ text: async () => fs.readFileSync(second, "utf8") });
globalThis.EventSource = class {
    constructor() { globalThis.listeners = {}; }
    addEventListener(name, fn) { globalThis.listeners[name] = fn; }
    close() {}
};
globalThis.setTimeout = () => {};
globalThis.console = console;

// The identity of one element proves the morph: a reload keeps it, and a
// navigation would build a new document.
const before = document.childNodes[0];

eval(fs.readFileSync(process.argv[4], "utf8"));

globalThis.listeners.reload();

setImmediate(() => {
    const after = document.childNodes[0];
    const result = {
        scrollY: state.scrollY,
        keptIdentity: before === after,
        text: document.text.replace(/\s+/g, " ").trim(),
    };
    process.stdout.write(JSON.stringify(result));
});
