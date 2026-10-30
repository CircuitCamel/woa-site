// Live summary for the character builder. The server does all of the calculating,
// so rules such as the point buy total exist only in one place. Without this file
// the summary updates when the form is submitted.
(function () {
    const form = document.getElementById('builder-form');
    const preview = document.getElementById('builder-preview');
    if (!form || !preview) return;

    const DELAY = 200;
    let timer;
    let controller;

    async function refresh() {
        if (controller) controller.abort();
        controller = new AbortController();
        try {
            const res = await fetch('/sheets/new/preview', {
                method: 'POST',
                body: new URLSearchParams(new FormData(form)),
                signal: controller.signal,
            });
            if (res.redirected && new URL(res.url).pathname === '/login') {
                window.location.assign(res.url);
                return;
            }
            if (!res.ok) return;
            const body = new DOMParser().parseFromString(await res.text(), 'text/html').body;
            preview.replaceChildren(...body.childNodes);
        } catch (err) {
            // The summary is a convenience. Saving still validates everything.
        }
    }

    function schedule() {
        clearTimeout(timer);
        timer = setTimeout(refresh, DELAY);
    }

    form.addEventListener('input', schedule);
    form.addEventListener('change', schedule);
}());
