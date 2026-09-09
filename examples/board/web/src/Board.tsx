import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
    tasksCreateMutation,
    tasksDeleteMutation,
    tasksListOptions,
    tasksListQueryKey,
    tasksUpdateMutation,
} from "./client/@tanstack/react-query.gen";
import type { Task } from "./client";

// Board reads the tasks of the API and writes them.
//
// Every call comes from src/client, which `avero build` writes from the
// description of the API. A change to a Go handler therefore reaches this file
// as a type fault, and no shape is written two times.
export function Board() {
    const queries = useQueryClient();
    const [title, setTitle] = useState("");
    const [fault, setFault] = useState("");

    const tasks = useQuery(tasksListOptions());
    const invalidate = () => {
        void queries.invalidateQueries({ queryKey: tasksListQueryKey() });
    };

    const create = useMutation({
        ...tasksCreateMutation(),
        onError: () => setFault("The task does not save. Write a title."),
        onSuccess: () => {
            setFault("");
            setTitle("");
            invalidate();
        },
    });
    const toggle = useMutation({ ...tasksUpdateMutation(), onSuccess: invalidate });
    const remove = useMutation({ ...tasksDeleteMutation(), onSuccess: invalidate });

    const submit = (event: FormEvent) => {
        event.preventDefault();
        create.mutate({ body: { title } });
    };

    if (tasks.isPending) {
        return <p>The board loads.</p>;
    }
    if (tasks.isError) {
        return <p className="error">The board does not load: {tasks.error.message}</p>;
    }

    const rows: Task[] = tasks.data?.tasks ?? [];

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

            {rows.length === 0 && <p className="empty">No task exists yet.</p>}

            <ul className="tasks">
                {rows.map((task) => (
                    <li key={task.id} className={task.done ? "done" : ""}>
                        <label>
                            <input
                                type="checkbox"
                                checked={task.done}
                                onChange={() =>
                                    toggle.mutate({ path: { id: task.id }, body: { done: !task.done } })
                                }
                            />
                            {task.title}
                        </label>
                        <button type="button" onClick={() => remove.mutate({ path: { id: task.id } })}>
                            Delete
                        </button>
                    </li>
                ))}
            </ul>
        </>
    );
}
