// The client of the JSON API of the application.
//
// The Go handlers state the shape of each answer, and these types state the
// same shape for the front end. `avero routes --openapi` writes the
// description of the API, which a generator can read for a larger service.

export type Task = {
    id: string;
    title: string;
    done: boolean;
};

export type TaskList = {
    tasks: Task[];
};

export type FieldErrors = {
    errors: Record<string, string>;
};

// RequestFault carries the status and the field errors of a failed request.
export class RequestFault extends Error {
    readonly status: number;
    readonly fields: Record<string, string>;

    constructor(status: number, fields: Record<string, string>, message: string) {
        super(message);
        this.status = status;
        this.fields = fields;
    }
}

// read calls the API and returns the answer.
async function read<T>(path: string, options: RequestInit = {}): Promise<T> {
    const answer = await fetch(path, {
        headers: { "Content-Type": "application/json" },
        ...options,
    });
    if (!answer.ok) {
        let fields: Record<string, string> = {};
        try {
            fields = ((await answer.json()) as FieldErrors).errors ?? {};
        } catch {
            // The answer carries no field errors.
        }
        throw new RequestFault(answer.status, fields, `the request answered ${answer.status}`);
    }
    if (answer.status === 204) {
        return undefined as T;
    }
    return (await answer.json()) as T;
}

export const api = {
    listTasks: () => read<TaskList>("/api/tasks"),
    createTask: (title: string) =>
        read<Task>("/api/tasks", { method: "POST", body: JSON.stringify({ title }) }),
    setDone: (id: string, done: boolean) =>
        read<Task>(`/api/tasks/${id}`, { method: "PATCH", body: JSON.stringify({ done }) }),
    deleteTask: (id: string) => read<void>(`/api/tasks/${id}`, { method: "DELETE" }),
};
