# Static Files Placeholder

This directory is used to embed the frontend build output for the web server.

During Docker build, the frontend is built and copied here before Go compilation.

For local development, either:
1. Run `cd frontend && npm run build` and copy `dist/*` here
2. Or the server will fallback to serving from `./frontend/dist` directly
