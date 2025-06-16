const path = location.pathname.split('/').filter(Boolean);
const breadcrumb = document.getElementById('breadcrumb')

// Always start with Home
let breadcrumbItems = ['<a href="/">Home</a>'];

let url = '';

for (let i = 0; i < path.length; i++) {
    url += '/' + path[i];
    // Decode, replace underscores, then capitalize each word
    let text = decodeURIComponent(path[i]).replace(/_/g, ' ').split(' ').map(function(word) {
        return word.charAt(0).toUpperCase() + word.slice(1);
    }).join(' ');

    if (i === path.length - 1) {
        breadcrumbItems.push(`<span aria-current="page">${text}</span>`);
    } else {
        breadcrumbItems.push(`<a href="${url}">${text}</a>`);
    }
}
breadcrumb.innerHTML = breadcrumbItems.join(' / ');