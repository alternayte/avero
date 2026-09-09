import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// The build writes into assets/dist, which the Go application embeds. One
// binary therefore holds the server and the front end.
//
// The development server proxies the API to the Go application. `avero dev`
// sets AVERO_API_URL to the address that it gave the application, and Vite
// reads it from the environment.
export default defineConfig(({ mode }) => {
    const env = loadEnv(mode, ".", "AVERO_");
    const api = env.AVERO_API_URL ?? "http://127.0.0.1:8080";

    return {
        plugins: [react(), tailwindcss()],
        build: {
            outDir: "../assets/dist",
            emptyOutDir: true,
            sourcemap: true,
        },
        server: {
            port: 5173,
            strictPort: false,
            proxy: {
                "/api": { target: api, changeOrigin: true },
            },
        },
    };
});
