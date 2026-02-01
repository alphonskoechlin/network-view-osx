# Fix Summary: 404 Error on Root Path

## Issue
HTTP requests to `http://192.168.98.140:9999/` returned `HTTP/1.1 404 Not Found` instead of serving the frontend HTML page.

## Root Cause
The backend was using Go's `http.FileServer` to serve frontend files, but `http.FileServer` doesn't know how to handle the root path `/`. When a request came in for `/`, the file server looked for a file named `/` in the dist directory, couldn't find it, and returned 404.

The problematic code was:
```go
fs := http.FileServer(http.Dir(distPath))
mux.Handle("/", fs)
```

This works fine for actual files (`/assets/index.js`, `/vite.svg`, etc.) but fails for SPA (Single Page Application) routing where the app should handle all undefined routes.

## Solution
Implemented a custom HTTP handler function that:

1. **Routes API calls correctly**: Requests to `/api/*`, `/discover`, and `/health` are properly routed (pass through or return 404)
2. **Serves existing files**: If a file exists in the dist directory, serve it directly
3. **Serves index.html for root**: When `/` is requested, serve `index.html`
4. **Serves index.html for SPA fallback**: When a route doesn't exist as a file, serve `index.html` so the frontend app can handle routing
5. **Security**: Prevents directory traversal attacks by validating the full path

### Code Changes
File: `backend/main.go`

```go
// Before:
fs := http.FileServer(http.Dir(distPath))
mux.Handle("/", fs)

// After:
mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    // API routes should be handled by their specific handlers
    if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/discover") || strings.HasPrefix(r.URL.Path, "/health") {
        http.NotFound(w, r)
        return
    }
    
    // Try to serve the requested file
    fullPath := filepath.Join(distPath, filepath.Clean(r.URL.Path))
    
    // Security: prevent directory traversal
    if !strings.HasPrefix(fullPath, distPath) {
        http.NotFound(w, r)
        return
    }
    
    // Check if file exists
    if _, err := os.Stat(fullPath); err == nil {
        // File exists, serve it
        http.ServeFile(w, r, fullPath)
    } else if r.URL.Path == "/" {
        // Root path, serve index.html
        http.ServeFile(w, r, filepath.Join(distPath, "index.html"))
    } else {
        // File doesn't exist, serve index.html (for SPA routing)
        w.Header().Set("Content-Type", "text/html")
        http.ServeFile(w, r, filepath.Join(distPath, "index.html"))
    }
})
```

## Testing
Local testing on port 8888 confirms the fix works:

### Before Fix
```
$ curl -i http://127.0.0.1:8888/
HTTP/1.1 404 Not Found
...
404 page not found
```

### After Fix
```
$ curl -i http://127.0.0.1:8888/
HTTP/1.1 200 OK
Content-Type: text/html
...
<!doctype html>
<html lang="en">
  <head>
    <title>Network View macOS</title>
    ...
```

### API Endpoints Still Work
```
$ curl http://127.0.0.1:8888/health
{"status":"ok"}

$ curl http://127.0.0.1:8888/api/interfaces
{"current":"lo0","interfaces":[...]}
```

## Deployment
1. Rebuild the backend: `make backend`
2. Restart the server with the new binary
3. Verify: `curl http://192.168.98.140:9999/` should return 200 OK

The fixed binary is in `backend/network-view-osx` and the code is committed to `main` branch on GitHub.
