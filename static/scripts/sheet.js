// Sheet enhancements. Every action is a normal form the server handles, so the
// page still works without this file. Nothing here calculates a game value: the
// server re-renders the sheet and this swaps it in. Player text only ever reaches
// the page through server-escaped HTML or textContent.
(function () {
    const root = document.getElementById('sheet-root');
    if (!root) return;

    const dialog = document.getElementById('detail-dialog');
    const dialogBody = document.getElementById('detail-dialog-body');
    const toasts = document.getElementById('toasts');
    const AUTOSAVE_DELAY = 800;
    const TOAST_TIME = 6000;

    let currentTab = root.dataset.tab;
    let lastRoll = rollID();

    function rollID() {
        const el = document.getElementById('last-roll');
        return el ? el.dataset.roll : '';
    }

    function toast(label, total, detail, kind) {
        const box = document.createElement('div');
        box.className = 'toast' + (kind ? ' toast-' + kind : '');
        const name = document.createElement('span');
        name.textContent = label;
        box.append(name);
        if (total) {
            const big = document.createElement('strong');
            big.textContent = total;
            box.append(big);
        }
        if (detail) {
            const small = document.createElement('span');
            small.className = 'hint';
            small.textContent = detail;
            box.append(small);
        }
        toasts.append(box);
        setTimeout(() => box.remove(), TOAST_TIME);
    }

    function showRollToast() {
        const el = document.getElementById('last-roll');
        if (!el || el.dataset.roll === lastRoll) return;
        lastRoll = el.dataset.roll;
        const part = (cls) => {
            const node = el.querySelector(cls);
            return node ? node.textContent : '';
        };
        toast(part('.last-roll-label'), part('.last-roll-total'), part('.last-roll-detail'));
    }

    // A session can end while the page is open. The server redirects those
    // requests, and the browser should follow rather than swap in a login page.
    function leftTheSheet(res) {
        const path = new URL(res.url).pathname;
        if (res.redirected && (path === '/login' || path === '/account')) {
            window.location.assign(res.url);
            return true;
        }
        return false;
    }

    function parseHTML(text) {
        return new DOMParser().parseFromString(text, 'text/html').body;
    }

    function setTab(tab) {
        currentTab = tab;
        root.dataset.tab = tab;
        root.querySelectorAll('[data-panel]').forEach((p) => p.classList.toggle('active', p.dataset.panel === tab));
        root.querySelectorAll('[data-tab-link]').forEach((a) => {
            const on = a.dataset.tabLink === tab;
            a.classList.toggle('active', on);
            if (on) a.setAttribute('aria-current', 'page');
            else a.removeAttribute('aria-current');
        });
    }

    function addSteppers() {
        root.querySelectorAll('input[data-stepper]:not([data-stepped])').forEach((input) => {
            input.dataset.stepped = '1';
            const make = (text, label, fn) => {
                const b = document.createElement('button');
                b.type = 'button';
                b.className = 'chip chip-small';
                b.textContent = text;
                b.setAttribute('aria-label', label);
                b.addEventListener('click', () => {
                    fn();
                    input.dispatchEvent(new Event('input', { bubbles: true }));
                });
                return b;
            };
            input.before(make('−', 'Decrease', () => input.stepDown()));
            input.after(make('+', 'Increase', () => input.stepUp()));
        });
    }

    // A form's identity across a swap: where it posts and its hidden values.
    function formKey(form) {
        const hidden = [...form.querySelectorAll('input[type=hidden]')]
            .filter((i) => i.name !== 'tab')
            .map((i) => i.name + '=' + i.value);
        return form.getAttribute('action') + '|' + hidden.join('&');
    }

    function restoreFocus(key, submitter) {
        if (!submitter) return;
        const form = [...root.querySelectorAll('form[action]')].find((f) => formKey(f) === key);
        if (!form) return;
        const match = [...form.querySelectorAll('button')].find((b) => b.name === submitter.name && b.value === submitter.value);
        if (match && !match.disabled) match.focus();
    }

    // Replace the sheet, keeping what lives only in the browser: search results
    // and text that hasn't reached the server yet.
    function swap(body) {
        const search = {};
        root.querySelectorAll('form[data-search]').forEach((f) => {
            search[f.dataset.results] = {
                values: new FormData(f),
                results: document.getElementById(f.dataset.results).cloneNode(true),
            };
        });
        const unsaved = {};
        root.querySelectorAll('form[data-autosave][data-dirty]').forEach((f) => {
            unsaved[autosaveKey(f)] = new FormData(f);
        });

        root.replaceChildren(...body.childNodes);

        root.querySelectorAll('form[data-search]').forEach((f) => {
            const old = search[f.dataset.results];
            if (!old) return;
            for (const [name, value] of old.values) {
                if (f.elements[name] && name !== 'tab') f.elements[name].value = value;
            }
            document.getElementById(f.dataset.results).replaceWith(old.results);
        });
        root.querySelectorAll('form[data-autosave]').forEach((f) => {
            const values = unsaved[autosaveKey(f)];
            if (!values) return;
            for (const [name, value] of values) {
                if (f.elements[name] && name !== 'tab') f.elements[name].value = value;
            }
            f.dataset.dirty = '1';
            scheduleSave(f);
        });

        setTab(currentTab);
        addSteppers();
    }

    async function submitForm(form, submitter) {
        const data = new FormData(form, submitter);
        data.set('tab', currentTab);
        const key = formKey(form);
        await flushSaves();
        let res;
        try {
            res = await fetch(form.getAttribute('action'), {
                method: 'POST',
                body: new URLSearchParams(data),
                headers: { 'X-Partial': '1' },
            });
        } catch (err) {
            toast('Couldn\'t reach the server. Nothing was saved.', '', '', 'error');
            return;
        }
        if (leftTheSheet(res)) return;
        const text = await res.text();
        if (res.status === 400) {
            // Don't swap, so that whatever the player typed stays in the form.
            const banner = parseHTML(text).querySelector('.banner-error');
            toast(banner ? banner.textContent : 'That didn\'t work.', '', '', 'error');
            return;
        }
        if (!res.ok) {
            toast('Something went wrong. Nothing was saved.', '', '', 'error');
            return;
        }
        swap(parseHTML(text));
        restoreFocus(key, submitter);
        showRollToast();
    }

    root.addEventListener('submit', (e) => {
        const form = e.target;
        if (form.method !== 'post' || 'noSwap' in form.dataset) return;
        e.preventDefault();
        if (form.dataset.confirm && !window.confirm(form.dataset.confirm)) return;
        if ('autosave' in form.dataset) {
            saveNow(form);
            return;
        }
        submitForm(form, e.submitter);
    });

    root.addEventListener('click', (e) => {
        if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
        const link = e.target.closest('[data-tab-link]');
        if (link) {
            e.preventDefault();
            setTab(link.dataset.tabLink);
            history.pushState({ tab: currentTab }, '', link.getAttribute('href'));
            return;
        }
        const summary = e.target.closest('details[data-dialog] > summary');
        if (summary && dialog.showModal) {
            e.preventDefault();
            const content = summary.parentElement.querySelector('.detail-body');
            dialogBody.replaceChildren(content.cloneNode(true));
            dialog.showModal();
        }
    });

    dialog.addEventListener('click', (e) => {
        if (e.target === dialog) dialog.close();
    });

    window.addEventListener('popstate', () => {
        setTab(new URLSearchParams(location.search).get('tab') || 'actions');
    });

    // Autosave for the free-text forms.
    const timers = new Map();

    function autosaveKey(form) {
        return form.querySelector('textarea').name;
    }

    function setState(form, text, failed) {
        const el = form.querySelector('[data-save-state]');
        if (!el) return;
        el.textContent = text;
        el.classList.toggle('save-failed', Boolean(failed));
    }

    function scheduleSave(form) {
        clearTimeout(timers.get(form));
        setState(form, 'Unsaved changes');
        timers.set(form, setTimeout(() => saveNow(form), AUTOSAVE_DELAY));
    }

    async function saveNow(form) {
        clearTimeout(timers.get(form));
        timers.delete(form);
        const data = new FormData(form);
        const sent = new URLSearchParams(data).toString();
        setState(form, 'Saving');
        try {
            const res = await fetch(form.getAttribute('action'), {
                method: 'POST',
                body: new URLSearchParams(data),
                headers: { 'X-Autosave': '1' },
            });
            if (leftTheSheet(res)) return;
            if (!res.ok) throw new Error(await res.text());
            // Stay dirty if the player kept typing while this was in flight.
            if (new URLSearchParams(new FormData(form)).toString() === sent) {
                delete form.dataset.dirty;
                setState(form, 'Saved');
            }
        } catch (err) {
            setState(form, 'Couldn\'t save. Your text is still here, so try again.', true);
        }
    }

    function flushSaves() {
        const pending = [...timers.keys()];
        return Promise.all(pending.map(saveNow));
    }

    root.addEventListener('input', (e) => {
        const form = e.target.form;
        if (form && 'autosave' in form.dataset) {
            form.dataset.dirty = '1';
            scheduleSave(form);
        }
    });

    // Last chance when leaving the page. A beacon outlives the page.
    window.addEventListener('pagehide', () => {
        root.querySelectorAll('form[data-autosave][data-dirty]').forEach((form) => {
            navigator.sendBeacon(form.getAttribute('action'), new URLSearchParams(new FormData(form)));
        });
    });

    addSteppers();
}());
