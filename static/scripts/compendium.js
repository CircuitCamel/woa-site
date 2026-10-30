// Live spell and item search. Results come from the server as escaped HTML. The
// same forms work as a normal search without this file.
(function () {
    const root = document.getElementById('sheet-root');
    if (!root) return;

    const DELAY = 250;
    const running = new Map();
    const timers = new Map();

    async function search(form) {
        const params = new URLSearchParams(new FormData(form));
        params.delete('tab');
        params.set('sheet', root.dataset.sheet);

        const key = form.dataset.results;
        if (running.has(key)) running.get(key).abort();
        const controller = new AbortController();
        running.set(key, controller);

        try {
            const res = await fetch(form.dataset.search + '?' + params.toString(), { signal: controller.signal });
            if (res.redirected && new URL(res.url).pathname === '/login') {
                window.location.assign(res.url);
                return;
            }
            if (!res.ok) throw new Error(res.statusText);
            const body = new DOMParser().parseFromString(await res.text(), 'text/html').body;
            // Forms swap the whole sheet, so look this up fresh every time.
            document.getElementById(key).replaceChildren(...body.childNodes);
        } catch (err) {
            if (err.name === 'AbortError') return;
            const p = document.createElement('p');
            p.className = 'hint save-failed';
            p.textContent = 'Search failed. Try again.';
            document.getElementById(key).replaceChildren(p);
        }
    }

    function onChange(e) {
        const form = e.target.form;
        if (!form || !form.dataset.search) return;
        clearTimeout(timers.get(form));
        timers.set(form, setTimeout(() => search(form), DELAY));
    }

    root.addEventListener('input', onChange);
    root.addEventListener('change', onChange);
    root.addEventListener('submit', (e) => {
        if (e.target.dataset.search) {
            e.preventDefault();
            search(e.target);
        }
    });
}());
