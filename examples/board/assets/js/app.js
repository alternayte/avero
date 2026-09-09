// The front end of the application. It reads the JSON API and renders the
// list of posts.
//
// `avero build` bundles this file with esbuild. Run `avero js pin <package>`
// to add a module, and import it by its bare name.

const app = document.getElementById("app");

async function load() {
    const answer = await fetch("/api/posts");
    if (!answer.ok) {
        app.textContent = "The list does not load.";
        return;
    }
    const body = await answer.json();
    render(body.posts ?? []);
}

function render(posts) {
    app.replaceChildren();
    const heading = document.createElement("h1");
    heading.textContent = "Posts";
    app.append(heading);

    if (posts.length === 0) {
        const empty = document.createElement("p");
        empty.textContent = "No post exists yet.";
        app.append(empty);
        return;
    }
    const list = document.createElement("ul");
    for (const post of posts) {
        const item = document.createElement("li");
        item.textContent = post.title;
        list.append(item);
    }
    app.append(list);
}

load();
