// The front end of the application.
//
// React and TanStack Query are vendored: `avero js pin` fetched each one as a
// bundled ES module and recorded its address and its hash in avero.lock.
// esbuild bundles this file with them, so the browser loads one script and the
// machine needs no Node.js. See DX-9.
import { useState } from "react";
import { createRoot } from "react-dom/client";
import {
    QueryClient,
    QueryClientProvider,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";

const client = new QueryClient();

// read calls the JSON API of the application.
async function read(path, options) {
    const answer = await fetch(path, {
        headers: { "Content-Type": "application/json" },
        ...options,
    });
    if (!answer.ok) {
        throw new Error(`the request answered ${answer.status}`);
    }
    if (answer.status === 204) {
        return null;
    }
    return answer.json();
}

function Board() {
    const queries = useQueryClient();
    const [title, setTitle] = useState("");

    const tasks = useQuery({
        queryKey: ["tasks"],
        queryFn: () => read("/api/tasks"),
    });

    const invalidate = { onSuccess: () => queries.invalidateQueries({ queryKey: ["tasks"] }) };

    const create = useMutation({
        mutationFn: (value) => read("/api/tasks", { method: "POST", body: JSON.stringify({ title: value }) }),
        ...invalidate,
    });
    const toggle = useMutation({
        mutationFn: (task) =>
            read(`/api/tasks/${task.id}`, { method: "PATCH", body: JSON.stringify({ done: !task.done }) }),
        ...invalidate,
    });
    const remove = useMutation({
        mutationFn: (task) => read(`/api/tasks/${task.id}`, { method: "DELETE" }),
        ...invalidate,
    });

    if (tasks.isPending) {
        return <p>The board loads.</p>;
    }
    if (tasks.isError) {
        return <p className="error">The board does not load: {tasks.error.message}</p>;
    }

    return (
        <>
            <h1>Board</h1>
            <form
                onSubmit={(event) => {
                    event.preventDefault();
                    if (title.trim() === "") {
                        return;
                    }
                    create.mutate(title);
                    setTitle("");
                }}
            >
                <input
                    aria-label="Title"
                    value={title}
                    onChange={(event) => setTitle(event.target.value)}
                    placeholder="Write a task"
                />
                <button type="submit" disabled={create.isPending}>Add</button>
            </form>

            <ul className="tasks">
                {tasks.data.tasks.map((task) => (
                    <li key={task.id} className={task.done ? "done" : ""}>
                        <label>
                            <input type="checkbox" checked={task.done} onChange={() => toggle.mutate(task)} />
                            {task.title}
                        </label>
                        <button type="button" onClick={() => remove.mutate(task)}>Delete</button>
                    </li>
                ))}
            </ul>
            {tasks.data.tasks.length === 0 && <p className="empty">No task exists yet.</p>}
        </>
    );
}

createRoot(document.getElementById("app")).render(
    <QueryClientProvider client={client}>
        <Board />
    </QueryClientProvider>,
);
