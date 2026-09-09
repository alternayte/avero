// The reload client of `avero dev`. The proxy adds it to an HTML answer, and
// only when AVERO_ENV=development. A production build carries none of it.
(() => {
    // A reload fetches the page and morphs the document. The page therefore
    // keeps its scroll position, its focus and the state of its elements
    // across a restart of the application. See S15.
    const morphAttributes = (from, to) => {
        for (const attribute of Array.from(from.attributes)) {
            if (!to.hasAttribute(attribute.name)) {
                from.removeAttribute(attribute.name);
            }
        }
        for (const attribute of Array.from(to.attributes)) {
            if (from.getAttribute(attribute.name) !== attribute.value) {
                from.setAttribute(attribute.name, attribute.value);
            }
        }
    };

    const key = (node) => {
        if (node.nodeType !== Node.ELEMENT_NODE) {
            return null;
        }
        return node.id ? node.tagName + "#" + node.id : null;
    };

    const morph = (from, to) => {
        if (from.nodeType !== to.nodeType || from.nodeName !== to.nodeName) {
            from.replaceWith(to.cloneNode(true));
            return;
        }
        if (from.nodeType === Node.TEXT_NODE || from.nodeType === Node.COMMENT_NODE) {
            if (from.nodeValue !== to.nodeValue) {
                from.nodeValue = to.nodeValue;
            }
            return;
        }
        if (from.nodeType !== Node.ELEMENT_NODE) {
            return;
        }
        morphAttributes(from, to);

        // An element that a person is typing in keeps its value.
        if (from === document.activeElement && "value" in from) {
            return;
        }

        const byKey = new Map();
        for (const child of Array.from(from.childNodes)) {
            const id = key(child);
            if (id !== null) {
                byKey.set(id, child);
            }
        }

        let current = from.firstChild;
        for (const next of Array.from(to.childNodes)) {
            const id = key(next);
            const kept = id !== null ? byKey.get(id) : null;
            if (kept) {
                if (kept !== current) {
                    from.insertBefore(kept, current);
                } else {
                    current = current.nextSibling;
                }
                morph(kept, next);
                byKey.delete(id);
                continue;
            }
            if (current) {
                const after = current.nextSibling;
                morph(current, next);
                current = after;
                continue;
            }
            from.appendChild(next.cloneNode(true));
        }
        while (current) {
            const after = current.nextSibling;
            current.remove();
            current = after;
        }
    };

    const reload = async () => {
        const answer = await fetch(window.location.href, {
            headers: { "X-Avero-Reload": "true" },
            cache: "no-store",
        });
        const text = await answer.text();
        const next = new DOMParser().parseFromString(text, "text/html");
        morph(document.documentElement, next.documentElement);
    };

    // A CSS change swaps the link element. The page keeps its state, so
    // nothing reloads. See DX-2.
    const swap = (href) => {
        for (const link of document.querySelectorAll('link[rel="stylesheet"]')) {
            const next = link.cloneNode();
            next.href = href;
            next.addEventListener("load", () => link.remove(), { once: true });
            link.parentNode.insertBefore(next, link.nextSibling);
        }
    };

    const open = () => {
        const source = new EventSource("/_avero/reload");
        source.addEventListener("css", (message) => {
            swap(JSON.parse(message.data).href);
        });
        source.addEventListener("reload", () => {
            reload().catch((fault) => console.error("avero dev:", fault));
        });
        source.addEventListener("script", () => {
            // A module that already runs cannot be replaced, so the page
            // loads again. Every other change morphs the document.
            window.location.reload();
        });
        source.addEventListener("fault", (message) => {
            console.error("avero dev:", JSON.parse(message.data).message);
        });
        source.addEventListener("error", () => {
            // The application restarts. The client opens the channel again.
            source.close();
            setTimeout(open, 200);
        });
    };
    open();
})();
