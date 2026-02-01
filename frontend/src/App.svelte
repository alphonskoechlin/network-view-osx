<script>
  import { onMount } from 'svelte';
  import {
    DataTable,
    Toolbar,
    ToolbarContent,
    ToolbarSearch,
    DataTableSkeleton,
    Button,
    Pagination,
    InlineNotification
  } from 'carbon-components-svelte';
  import { Renew, Network } from 'carbon-icons-svelte';
  import './App.css';

  let services = [];
  let loading = false;
  let error = null;
  let pageSize = 10;
  let currentPage = 0;
  let searchQuery = '';
  let connected = false;
  let eventSource = null;

  const headers = [
    { key: 'name', value: 'Service Name' },
    { key: 'type', value: 'Type' },
    { key: 'host', value: 'Host' },
    { key: 'ip', value: 'IP Address' },
    { key: 'port', value: 'Port' }
  ];

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

    // Close previous connection if exists
    if (eventSource) {
      eventSource.close();
    }

    eventSource = new EventSource('http://localhost:8080/discover');

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
        error = 'Connection to mDNS service lost. Make sure backend is running on :8080';
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
      <div class="header-title">
        <Network size={32} />
        <h1>Network View macOS</h1>
      </div>
      <div class="connection-status" class:connected>
        <span class="status-dot"></span>
        {connected ? 'Connected' : 'Disconnected'}
      </div>
    </div>
  </header>

  <main class="app-main">
    <div class="content-wrapper">
      {#if error}
        <InlineNotification kind="error" title="Connection Error">
          {error}
          <Button size="small" kind="secondary" on:click={reconnect}>
            Retry
          </Button>
        </InlineNotification>
      {/if}

      {#if loading && services.length === 0}
        <DataTableSkeleton {headers} rows={5} />
      {:else}
        <div class="table-wrapper">
          <Toolbar>
            <ToolbarContent>
              <ToolbarSearch
                placeholder="Search services..."
                bind:value={searchQuery}
              />
              <Button
                icon={Renew}
                kind="secondary"
                on:click={reconnect}
                disabled={connected && !error}
              >
                Reconnect
              </Button>
            </ToolbarContent>
          </Toolbar>

          <DataTable
            {headers}
            rows={filteredRows}
            pageSize={pageSize}
            bind:pageSize
            bind:currentPageIndex={currentPage}
          />

          {#if filteredRows.length > 0}
            <div class="table-footer">
              <span class="service-count">
                {filteredRows.length} service{filteredRows.length !== 1 ? 's' : ''} discovered
              </span>
              <Pagination
                bind:pageSize
                bind:page={currentPage}
                totalItems={filteredRows.length}
                pageSizeOptions={[10, 20, 50]}
                on:update
              />
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

<style global>
  :global(body) {
    margin: 0;
    padding: 0;
    font-family: 'IBM Plex Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    background-color: #f4f4f4;
  }

  :global(.bx--data-table) {
    background-color: white;
  }

  :global(.bx--toolbar) {
    padding: 1rem;
    background-color: white;
  }

  :global(.bx--inline-notification) {
    margin-bottom: 1rem;
  }
</style>
