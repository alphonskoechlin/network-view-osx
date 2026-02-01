<script>
  import { onMount } from 'svelte';

  let services = [];
  let loading = false;
  let error = null;
  let searchQuery = '';
  let connected = false;
  let eventSource = null;
  let pageSize = 10;
  let currentPage = 0;

  let filteredRows = [];

  $: {
    let filtered = services;
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      filtered = services.filter(s =>
        s.name.toLowerCase().includes(query) ||
        s.type.toLowerCase().includes(query) ||
        s.host.toLowerCase().includes(query) ||
        s.ip.toLowerCase().includes(query)
      );
    }
    filteredRows = filtered;
  }

  function connectToMDNS() {
    loading = true;
    error = null;
    connected = false;

    if (eventSource) {
      eventSource.close();
    }

    eventSource = new EventSource('http://192.168.98.140:9999/discover');

    eventSource.onopen = () => {
      connected = true;
      loading = false;
      console.log('Connected to mDNS stream');
    };

    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.service && !data.removed) {
          const service = data.service;
          const id = `${service.ip}:${service.port}:${service.type}`;
          const existingIndex = services.findIndex(s => `${s.ip}:${s.port}:${s.type}` === id);
          
          if (existingIndex === -1) {
            services = [...services, {
              id,
              name: service.name,
              type: service.type,
              host: service.host,
              ip: service.ip,
              port: service.port.toString()
            }];
          }
        }
      } catch (e) {
        console.error('Error parsing message:', e);
      }
    };

    eventSource.onerror = (err) => {
      console.error('EventSource error:', err);
      connected = false;
      loading = false;
      if (eventSource && eventSource.readyState === EventSource.CLOSED) {
        error = 'Connection to mDNS service lost. Make sure backend is running on :9999';
      }
    };
  }

  function reconnect() {
    error = null;
    connectToMDNS();
  }

  function clearServices() {
    services = [];
    filteredRows = [];
  }

  onMount(() => {
    connectToMDNS();

    return () => {
      if (eventSource) {
        eventSource.close();
      }
    };
  });
</script>

<div class="app-container">
  <header class="app-header">
    <div class="header-content">
      <h1>Network View macOS</h1>
      <div class="connection-status" class:connected>
        <span class="status-dot"></span>
        {connected ? 'Connected' : 'Disconnected'}
      </div>
    </div>
  </header>

  <main class="app-main">
    <div class="content-wrapper">
      {#if error}
        <div class="error-box">
          <p><strong>Connection Error:</strong> {error}</p>
          <button on:click={reconnect}>Retry</button>
        </div>
      {/if}

      {#if loading && services.length === 0}
        <p>Loading...</p>
      {:else}
        <div class="table-wrapper">
          <div class="controls">
            <input
              type="text"
              placeholder="Search services..."
              bind:value={searchQuery}
            />
            <button on:click={reconnect} disabled={connected && !error}>
              Reconnect
            </button>
          </div>

          {#if filteredRows.length > 0}
            <table>
              <thead>
                <tr>
                  <th>Service Name</th>
                  <th>Type</th>
                  <th>Host</th>
                  <th>IP Address</th>
                  <th>Port</th>
                </tr>
              </thead>
              <tbody>
                {#each filteredRows as service (service.id)}
                  <tr>
                    <td>{service.name}</td>
                    <td>{service.type}</td>
                    <td>{service.host}</td>
                    <td>{service.ip}</td>
                    <td>{service.port}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
            <div class="table-footer">
              <span class="service-count">
                {filteredRows.length} service{filteredRows.length !== 1 ? 's' : ''} discovered
              </span>
            </div>
          {:else}
            <div class="empty-state">
              <p>No services discovered yet...</p>
              <p class="help-text">
                {#if connected}
                  Waiting for mDNS announcements on the network
                {:else}
                  Waiting to connect to backend service
                {/if}
              </p>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </main>
</div>

<style>
  :global(body) {
    margin: 0;
    padding: 0;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen',
      'Ubuntu', 'Cantarell', 'Fira Sans', 'Droid Sans', 'Helvetica Neue', sans-serif;
    background-color: #f4f4f4;
  }

  .app-container {
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

  .app-header {
    background-color: #fff;
    border-bottom: 1px solid #e0e0e0;
    padding: 1.5rem;
  }

  .header-content {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .connection-status {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background-color: #f5f5f5;
    border-radius: 4px;
    font-size: 14px;
    font-weight: 500;
  }

  .connection-status.connected {
    background-color: #e8f5e9;
    color: #2e7d32;
  }

  .status-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background-color: #f44336;
  }

  .connection-status.connected .status-dot {
    background-color: #4caf50;
  }

  .app-main {
    flex: 1;
    overflow-y: auto;
    padding: 2rem;
  }

  .content-wrapper {
    max-width: 1200px;
    margin: 0 auto;
  }

  .error-box {
    background-color: #ffebee;
    border: 1px solid #f44336;
    border-radius: 4px;
    padding: 1rem;
    margin-bottom: 1rem;
  }

  .error-box button {
    margin-top: 0.5rem;
    padding: 6px 12px;
    background-color: #f44336;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }

  .error-box button:hover {
    background-color: #d32f2f;
  }

  .controls {
    display: flex;
    gap: 1rem;
    margin-bottom: 1rem;
  }

  .controls input {
    flex: 1;
    padding: 8px 12px;
    border: 1px solid #ddd;
    border-radius: 4px;
    font-size: 14px;
  }

  .controls button {
    padding: 8px 16px;
    background-color: #1976d2;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-weight: 500;
  }

  .controls button:hover {
    background-color: #1565c0;
  }

  .controls button:disabled {
    background-color: #ccc;
    cursor: not-allowed;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    background-color: white;
  }

  thead {
    background-color: #f5f5f5;
  }

  th {
    padding: 12px;
    text-align: left;
    font-weight: 600;
    border-bottom: 2px solid #ddd;
  }

  td {
    padding: 12px;
    border-bottom: 1px solid #eee;
  }

  tr:hover {
    background-color: #f9f9f9;
  }

  .table-footer {
    padding: 1rem;
    background-color: #f5f5f5;
    border-radius: 0 0 4px 4px;
    text-align: right;
    font-size: 14px;
    color: #666;
  }

  .empty-state {
    text-align: center;
    padding: 2rem;
    background-color: white;
    border-radius: 4px;
  }

  .empty-state p {
    color: #999;
  }

  .help-text {
    font-size: 14px;
    margin-top: 0.5rem;
  }
</style>
