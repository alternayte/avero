import { defineConfig } from "@hey-api/openapi-ts";

// The generator reads the description that `avero routes --openapi` writes,
// and it writes the client and the TanStack Query options of each route into
// src/client. Never edit that directory: change the Go handler and run
// `npm run api`.
export default defineConfig({
    input: "../openapi.json",
    // The output takes no formatter, so the generator needs no tool beside
    // the dependencies of the project.
    output: "src/client",
    plugins: [
        "@hey-api/client-fetch",
        {
            name: "@tanstack/react-query",
            queryOptions: true,
            mutationOptions: true,
        },
    ],
});
