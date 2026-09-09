import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api, RequestFault, type Task } from "./api";

// Board reads the tasks of the API and writes them.
export function Board() {
    const queries = useQueryClient();
    const [title, setTitle] = useState("");
    const [fault, setFault] = useState("");

    const tasks = useQuery({ queryKey: ["tasks"], queryFn: api.listTasks });
    const invalidate = {
        onSuccess: () => {
            void queries.invalidateQueries({ queryKey: ["tasks"] });
        },
    };

    const create = useMutation({
        mutationFn: api.createTask,
        onError: (error: unknown) => {
            // The Go handler answers 422 with one message for each field.
            setFault(error instanceof RequestFault ? (error.fields.title ?? error.message) : "the task does not save");
        },
        onSuccess: () => {
            setFault("");
            setTitle("");
            void queries.invalidateQueries({ queryKey: ["tasks"] });
        },
    });
    const toggle = useMutation({
        mutationFn: (task: Task) => api.setDone(task.id, !task.done),
        ...invalidate,
    });
    const remove = useMutation({
        mutationFn: (task: Task) => api.deleteTask(task.id),
        ...invalidate,
    });

    const submit = (event: FormEvent) => {
        event.preventDefault();
        create.mutate(title);
    };

    if (tasks.isPending) {
        return <p>The board loads.</p>;
    }
    if (tasks.isError) {
        return <p className="error">The board does not load: {tasks.error.message}</p>;
    }

    return (
        <>
            <h1>Board</h1>

            <form onSubmit={submit}>
                <input
                    aria-label="Title"
                    value={title}
                    onChange={(event) => setTitle(event.target.value)}
                    placeholder="Write a task"
                />
                <button type="submit" disabled={create.isPending}>
                    Add
                </button>
                {fault !== "" && <span className="error">{fault}</span>}
            </form>

            {tasks.data.tasks.length === 0 && <p className="empty">No task exists yet.</p>}

            <ul className="tasks">
                {tasks.data.tasks.map((task) => (
                    <li key={task.id} className={task.done ? "done" : ""}>
                        <label>
                            <input
                                type="checkbox"
                                checked={task.done}
                                onChange={() => toggle.mutate(task)}
                            />
                            {task.title}
                        </label>
                        <button type="button" onClick={() => remove.mutate(task)}>
                            Delete
                        </button>
                    </li>
                ))}
            </ul>
        </>
    );
}
