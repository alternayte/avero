import { defineConfig } from "@hey-api/openapi-ts";

// The generator reads the description that `avero routes --openapi` writes,
// and it writes the client and the TanStack Query options of each route into
// src/client. Never edit that directory: change the Go handler and run
// `npm run api`.
export default defineConfig({
    input: "../openapi.json",
    output: {
        path: "src/client",
        format: "prettier",
    },
    plugins: [
        "@hey-api/client-fetch",
        {
            name: "@tanstack/react-query",
            queryOptions: true,
            mutationOptions: true,
        },
    ],
});
