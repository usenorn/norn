<script lang="ts">
	import KeyRound from "@lucide/svelte/icons/key-round";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";

	let { workspace }: { workspace: { name: string; slug: string } } = $props();
</script>

<svelte:head><title>{workspace.name} requires single sign-on · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4 px-4 py-6">
	<div class="notch flex w-full max-w-115 flex-col gap-4 p-5 sm:p-6">
		<div class="flex items-start gap-2">
			<KeyRound class="mt-0.5 size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<div class="flex flex-col gap-1.5">
				<h1 class="text-xl font-medium tracking-title text-ink-900">
					{workspace.name} signs in through your provider
				</h1>
				<p class="text-md leading-normal text-ink-600 text-pretty">
					You are signed in with a Norn password, which no longer opens this workspace. Signing
					in through the provider gets you back in — everything here is still yours.
				</p>
			</div>
		</div>

		<div class="flex flex-col gap-2">
			<Eyebrow class="text-ink-600">What this does not affect</Eyebrow>
			<ul class="flex flex-col gap-2">
				{#each ["Your other workspaces keep working in this same session.", "Nothing has been removed from this workspace.", "API tokens and automation are unaffected."] as line (line)}
					<li class="flex items-start gap-2">
						<span class="mt-2 size-1 flex-none bg-muted-foreground" aria-hidden="true"></span>
						<span class="flex-1 text-md leading-normal text-ink-600">{line}</span>
					</li>
				{/each}
			</ul>
		</div>

		<div class="flex flex-wrap gap-2">
			<Button href="/sso?workspace={workspace.slug}">Continue with single sign-on</Button>
			<Button href="/" variant="secondary">Go to another workspace</Button>
		</div>
	</div>
</div>
