<script lang="ts">
	let { title, description }: { title: string; description?: string } = $props();
</script>

<svelte:head>
	<title>{title}</title>
	<meta name="robots" content="noindex" />
	<meta property="og:type" content="website" />
	<meta property="og:title" content={title} />
	<meta name="twitter:card" content="summary" />
	<meta name="twitter:title" content={title} />
	{#if description}
		<meta name="description" content={description} />
		<meta property="og:description" content={description} />
		<meta name="twitter:description" content={description} />
	{/if}
</svelte:head>
