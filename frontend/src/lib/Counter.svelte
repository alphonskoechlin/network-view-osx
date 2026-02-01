<script>
  let count = 0;
  const increment = () => {
    count += 1;
  }
</script>

<button on:click={increment}>
  count is {count}
</button>

<style>
  button {
    background-color: #646cff;
    border: none;
    border-radius: 8px;
    border: 1px solid transparent;
    padding: 0.6em 1.2em;
    font-size: 1em;
    font-weight: 500;
    font-family: inherit;
    background-color: #1a1a1a;
    cursor: pointer;
    transition: border-color 0.25s;
  }
  button:hover {
    border-color: #646cff;
  }
  button:focus,
  button:focus-visible {
    outline: 4px auto -webkit-focus-ring-color;
  }
</style>
