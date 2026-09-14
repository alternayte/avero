import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import { client } from "./client/client.gen";
import { Board } from "./Board";
import "./styles.css";

// The generated client reads the same origin, which Vite proxies to the Go
// application in development and the binary serves in production.
client.setConfig({ baseUrl: "/" });

const queries = new QueryClient();

const root = document.getElementById("root");
if (!root) {
    throw new Error("the document holds no root element");
}

createRoot(root).render(
    <StrictMode>
        <QueryClientProvider client={queries}>
            <main>
                <Board />
            </main>
        </QueryClientProvider>
    </StrictMode>,
);
